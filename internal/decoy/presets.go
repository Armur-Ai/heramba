package decoy

import "sort"

// A Preset is a framework-tailored bundle of decoy paths and decoy zones layered
// on top of the generic defaults. Choosing the preset that matches your stack is
// the biggest "afternoon-vs-week" lever in a deploy: the lures look native to
// your app (a Rails attacker probes /config/master.key, a Laravel attacker
// probes /storage/logs/laravel.log), so an agentic attacker enumerating
// "believable" paths for your framework walks straight into deception instead of
// finding clean 404s. Every path here is one no legitimate client should ever
// request, so detection stays zero-false-positive.
type Preset struct {
	Name     string
	Paths    []Endpoint   // exact-path lures
	Prefixes []Endpoint   // decoy zones (any path under the prefix is a hit)
	Seeds    []CanarySeed // believable fake secrets to plant + watch for this stack
}

// CanarySeed is a believable fake secret tailored to a framework — a value that
// looks exactly like the real thing (an AWS key, a Rails master.key, a Laravel
// APP_KEY) but is inert. You plant it where only an attacker who has broken in
// would find it (inside a decoy config file, a leaked-looking .env), and the
// edge watches for the value appearing in any later request. Because no
// legitimate client ever carries it, a hit is a zero-false-positive confirmation
// that the planted bait was taken. Every Value here is a documented/example-form
// token, never a live credential.
type CanarySeed struct {
	Plant string `json:"plant"` // where to plant it (which decoy file / field)
	Value string `json:"value"` // the believable fake secret value to seed and watch
}

var presets = map[string]Preset{
	"node": {
		Name: "node",
		Paths: []Endpoint{
			{"/.env.local", "Node env/secret file probe"},
			{"/.env.production", "Node production secrets probe"},
			{"/server.js", "Node entrypoint source probe"},
			{"/config/default.json", "node-config secrets probe"},
			{"/.npmrc", "npm registry token probe"},
			{"/npm-debug.log", "Node debug log probe"},
		},
		Prefixes: []Endpoint{
			{"/node_modules/.cache/", "Node build-cache walk"},
		},
		Seeds: []CanarySeed{
			{Plant: "/.npmrc (_authToken)", Value: "EXAMPLE-npm-authtoken-DEADBEEF0123456789abcd"},
			{Plant: "/.env.local (API_KEY)", Value: "EXAMPLE-apikey-0a1b2c3d4e5f60718293a4b5"},
		},
	},
	"next": {
		Name: "next",
		Paths: []Endpoint{
			{"/.env.local", "Next.js env/secret file probe"},
			{"/.env.production.local", "Next.js production secrets probe"},
			{"/next.config.js", "Next.js config source probe"},
			{"/_next/server/pages-manifest.json", "Next.js internal manifest probe"},
		},
		Prefixes: []Endpoint{
			{"/.next/cache/", "Next.js build-cache walk"},
		},
		Seeds: []CanarySeed{
			{Plant: "/.env.local (NEXT_PUBLIC_API_KEY)", Value: "nx_EXAMPLE0a1b2c3d4e5f60718293a4b5c6d7"},
		},
	},
	"rails": {
		Name: "rails",
		Paths: []Endpoint{
			{"/config/database.yml", "Rails database credential probe"},
			{"/config/secrets.yml", "Rails secrets probe"},
			{"/config/master.key", "Rails master key probe"},
			{"/config/credentials.yml.enc", "Rails encrypted credentials probe"},
			{"/log/production.log", "Rails production log probe"},
		},
		Prefixes: []Endpoint{
			{"/config/", "Rails config directory walk"},
		},
		Seeds: []CanarySeed{
			{Plant: "/config/master.key", Value: "0123456789abcdef0123456789abcdef"},
			{Plant: "/config/database.yml (password)", Value: "EXAMPLE_db_pw_7f3a9c21e4"},
		},
	},
	"django": {
		Name: "django",
		Paths: []Endpoint{
			{"/settings.py", "Django settings source probe"},
			{"/local_settings.py", "Django local settings probe"},
			{"/manage.py", "Django management entrypoint probe"},
			{"/db.sqlite3", "Django SQLite database probe"},
			{"/.env", "Django env/secret file probe"},
		},
		Seeds: []CanarySeed{
			{Plant: "/settings.py (SECRET_KEY)", Value: "django-insecure-EXAMPLE0a1b2c3d4e5f6071829"},
		},
	},
	"laravel": {
		Name: "laravel",
		Paths: []Endpoint{
			{"/.env", "Laravel env/secret file probe"},
			{"/.env.backup", "Laravel env backup probe"},
			{"/artisan", "Laravel console entrypoint probe"},
			{"/config/database.php", "Laravel database config probe"},
			{"/storage/logs/laravel.log", "Laravel log probe"},
		},
		Prefixes: []Endpoint{
			{"/storage/", "Laravel storage directory walk"},
		},
		Seeds: []CanarySeed{
			{Plant: "/.env (APP_KEY)", Value: "base64:EXAMPLEa0b1c2d3e4f5061728394a5b6c7d8e9f0012="},
			{Plant: "/.env (DB_PASSWORD)", Value: "EXAMPLE_laravel_pw_9c21e4"},
		},
	},
	"wordpress": {
		Name: "wordpress",
		Paths: []Endpoint{
			{"/wp-config.php", "WordPress config credential probe"},
			{"/wp-config.php.bak", "WordPress config backup probe"},
			{"/xmlrpc.php", "WordPress XML-RPC abuse probe"},
			{"/wp-content/debug.log", "WordPress debug log probe"},
		},
		Prefixes: []Endpoint{
			{"/wp-includes/", "WordPress core directory walk"},
		},
		Seeds: []CanarySeed{
			{Plant: "/wp-config.php (DB_PASSWORD)", Value: "EXAMPLE_wp_pw_3a9c21e4f7"},
			{Plant: "/wp-config.php (AUTH_KEY)", Value: "EXAMPLE-unique-phrase-7f3a9c21e4b6"},
		},
	},
	"fastapi": {
		Name: "fastapi",
		Paths: []Endpoint{
			{"/.env", "FastAPI env/secret file probe"},
			{"/app/core/config.py", "FastAPI settings source probe"},
			{"/alembic.ini", "Alembic DB migration config probe"},
			{"/.env.prod", "FastAPI production secrets probe"},
		},
		Seeds: []CanarySeed{
			{Plant: "/.env (SECRET_KEY)", Value: "EXAMPLE0a1b2c3d4e5f60718293a4b5c6d7e8f901"},
		},
	},
	"symfony": {
		Name: "symfony",
		Paths: []Endpoint{
			{"/.env", "Symfony env/secret file probe"},
			{"/.env.local", "Symfony local secrets probe"},
			{"/var/log/prod.log", "Symfony production log probe"},
			{"/config/packages/security.yaml", "Symfony security config probe"},
		},
		Prefixes: []Endpoint{
			{"/config/secrets/", "Symfony secrets directory walk"},
		},
		Seeds: []CanarySeed{
			{Plant: "/.env.local (APP_SECRET)", Value: "EXAMPLE0a1b2c3d4e5f60718293a4b5c6"},
		},
	},
	"aspnet": {
		Name: "aspnet",
		Paths: []Endpoint{
			{"/appsettings.json", "ASP.NET settings/credential probe"},
			{"/appsettings.Production.json", "ASP.NET production settings probe"},
			{"/web.config", "ASP.NET web.config probe"},
		},
		Prefixes: []Endpoint{
			{"/App_Data/", "ASP.NET App_Data directory walk"},
			{"/.vs/", "Visual Studio metadata walk"},
		},
		Seeds: []CanarySeed{
			{Plant: "/appsettings.json (ConnectionStrings password)", Value: "EXAMPLE_mssql_pw_7f3a9c21"},
		},
	},
	"go": {
		Name: "go",
		Paths: []Endpoint{
			{"/.env", "Go app env/secret file probe"},
			{"/config.yaml", "Go app config probe"},
			{"/app.env", "Go app env probe"},
		},
		Seeds: []CanarySeed{
			{Plant: "/.env (AWS_ACCESS_KEY_ID)", Value: "AKIAIOSFODNN7EXAMPLE"},
			{Plant: "/.env (AWS_SECRET_ACCESS_KEY)", Value: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"},
		},
	},
	"phoenix": {
		Name: "phoenix",
		Paths: []Endpoint{
			{"/.env", "Phoenix env/secret file probe"},
			{"/config/prod.secret.exs", "Phoenix production secrets probe"},
			{"/config/runtime.exs", "Phoenix runtime config probe"},
		},
		Seeds: []CanarySeed{
			{Plant: "/config/prod.secret.exs (secret_key_base)", Value: "EXAMPLE0a1b2c3d4e5f60718293a4b5c6d7e8f90112233445566778899aabbccddeeff00112233"},
		},
	},
	"spring": {
		Name: "spring",
		Paths: []Endpoint{
			{"/actuator/heapdump", "Spring Boot heap-dump probe"},
			{"/actuator/mappings", "Spring Boot route-enumeration probe"},
			{"/application.properties", "Spring config probe"},
			{"/application.yml", "Spring config probe"},
		},
		Prefixes: []Endpoint{
			{"/actuator/", "Spring Boot actuator enumeration"},
		},
		Seeds: []CanarySeed{
			{Plant: "/application.properties (datasource.password)", Value: "EXAMPLE_spring_pw_3a9c21e4"},
		},
	},
}

// PresetNames returns the available preset names, sorted.
func PresetNames() []string {
	names := make([]string, 0, len(presets))
	for n := range presets {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// GetPreset returns the named preset (ok=false if unknown).
func GetPreset(name string) (Preset, bool) {
	p, ok := presets[name]
	return p, ok
}

// ApplyPreset adds a preset's decoy paths and zones to the registry, and watches
// the preset's canary seeds so that if an attacker takes a planted bait secret
// and uses it, the edge detects it. Duplicates (e.g. a preset path already in
// the defaults) are harmless — the registry keys by path, so they coalesce.
func (r *Registry) ApplyPreset(p Preset) {
	for _, e := range p.Paths {
		r.Add(e.Path, e.Note)
	}
	for _, e := range p.Prefixes {
		r.AddPrefix(e.Path, e.Note)
	}
	for _, seed := range p.Seeds {
		r.canaries.Watch(seed.Value, "preset "+p.Name+" seed @ "+seed.Plant)
	}
}
