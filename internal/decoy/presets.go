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
	Paths    []Endpoint // exact-path lures
	Prefixes []Endpoint // decoy zones (any path under the prefix is a hit)
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
	},
	"fastapi": {
		Name: "fastapi",
		Paths: []Endpoint{
			{"/.env", "FastAPI env/secret file probe"},
			{"/app/core/config.py", "FastAPI settings source probe"},
			{"/alembic.ini", "Alembic DB migration config probe"},
			{"/.env.prod", "FastAPI production secrets probe"},
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
	},
	"go": {
		Name: "go",
		Paths: []Endpoint{
			{"/.env", "Go app env/secret file probe"},
			{"/config.yaml", "Go app config probe"},
			{"/app.env", "Go app env probe"},
		},
	},
	"phoenix": {
		Name: "phoenix",
		Paths: []Endpoint{
			{"/.env", "Phoenix env/secret file probe"},
			{"/config/prod.secret.exs", "Phoenix production secrets probe"},
			{"/config/runtime.exs", "Phoenix runtime config probe"},
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

// ApplyPreset adds a preset's decoy paths and zones to the registry. Duplicates
// (e.g. a preset path already in the defaults) are harmless — the registry keys
// by path, so they simply coalesce.
func (r *Registry) ApplyPreset(p Preset) {
	for _, e := range p.Paths {
		r.Add(e.Path, e.Note)
	}
	for _, e := range p.Prefixes {
		r.AddPrefix(e.Path, e.Note)
	}
}
