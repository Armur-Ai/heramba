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
	"drupal": {
		Name: "drupal",
		Paths: []Endpoint{
			{"/sites/default/settings.php", "Drupal settings/credential probe"},
			{"/sites/default/settings.local.php", "Drupal local settings probe"},
			{"/CHANGELOG.txt", "Drupal version-fingerprint probe"},
			{"/core/install.php", "Drupal installer probe"},
		},
		Prefixes: []Endpoint{
			{"/sites/default/files/", "Drupal files directory walk"},
		},
		Seeds: []CanarySeed{
			{Plant: "/sites/default/settings.php ($databases password)", Value: "EXAMPLE_drupal_pw_7f3a9c21"},
			{Plant: "/sites/default/settings.php (hash_salt)", Value: "EXAMPLE0a1b2c3d4e5f60718293a4b5c6d7e8f901"},
		},
	},
	"magento": {
		Name: "magento",
		Paths: []Endpoint{
			{"/app/etc/env.php", "Magento env/credential probe"},
			{"/app/etc/config.php", "Magento module config probe"},
			{"/app/etc/local.xml", "Magento 1 local config probe"},
			{"/var/log/system.log", "Magento system log probe"},
		},
		Prefixes: []Endpoint{
			{"/app/etc/", "Magento config directory walk"},
		},
		Seeds: []CanarySeed{
			{Plant: "/app/etc/env.php (crypt.key)", Value: "EXAMPLE0a1b2c3d4e5f60718293a4b5c6"},
			{Plant: "/app/etc/env.php (db password)", Value: "EXAMPLE_magento_pw_9c21e4"},
		},
	},
	"joomla": {
		Name: "joomla",
		Paths: []Endpoint{
			{"/configuration.php", "Joomla config/credential probe"},
			{"/configuration.php-dist", "Joomla config template probe"},
			{"/htaccess.txt", "Joomla default htaccess probe"},
			{"/administrator/manifests/files/joomla.xml", "Joomla version-fingerprint probe"},
		},
		Prefixes: []Endpoint{
			{"/administrator/", "Joomla admin enumeration"},
		},
		Seeds: []CanarySeed{
			{Plant: "/configuration.php ($password)", Value: "EXAMPLE_joomla_pw_3a9c21e4"},
			{Plant: "/configuration.php ($secret)", Value: "EXAMPLE0a1b2c3d4e5f6071"},
		},
	},
	"flask": {
		Name: "flask",
		Paths: []Endpoint{
			{"/.env", "Flask env/secret file probe"},
			{"/config.py", "Flask settings source probe"},
			{"/instance/config.py", "Flask instance secrets probe"},
			{"/app.py", "Flask entrypoint source probe"},
		},
		Seeds: []CanarySeed{
			{Plant: "/config.py (SECRET_KEY)", Value: "EXAMPLE0a1b2c3d4e5f60718293a4b5c6d7e8f901"},
		},
	},
	"express": {
		Name: "express",
		Paths: []Endpoint{
			{"/.env", "Express env/secret file probe"},
			{"/config/config.js", "Express config source probe"},
			{"/ecosystem.config.js", "PM2 process config probe"},
			{"/.sequelizerc", "Sequelize DB config probe"},
		},
		Seeds: []CanarySeed{
			{Plant: "/.env (SESSION_SECRET)", Value: "EXAMPLE-session-secret-0a1b2c3d4e5f6071"},
			{Plant: "/.env (JWT_SECRET)", Value: "EXAMPLE-jwt-secret-8293a4b5c6d7e8f9"},
		},
	},
	"nestjs": {
		Name: "nestjs",
		Paths: []Endpoint{
			{"/.env", "NestJS env/secret file probe"},
			{"/ormconfig.json", "TypeORM database credential probe"},
			{"/nest-cli.json", "NestJS project metadata probe"},
			{"/dist/main.js", "NestJS compiled entrypoint probe"},
		},
		Prefixes: []Endpoint{
			{"/dist/", "NestJS build-output walk"},
		},
		Seeds: []CanarySeed{
			{Plant: "/ormconfig.json (password)", Value: "EXAMPLE_nest_db_pw_7f3a9c21"},
			{Plant: "/.env (JWT_SECRET)", Value: "EXAMPLE0a1b2c3d4e5f60718293a4b5c6d7e8f901"},
		},
	},
	"strapi": {
		Name: "strapi",
		Paths: []Endpoint{
			{"/.env", "Strapi env/secret file probe"},
			{"/config/database.js", "Strapi database credential probe"},
			{"/config/admin.js", "Strapi admin JWT secret probe"},
			{"/config/server.js", "Strapi server/app-keys probe"},
		},
		Prefixes: []Endpoint{
			{"/config/", "Strapi config directory walk"},
		},
		Seeds: []CanarySeed{
			{Plant: "/config/admin.js (auth.secret)", Value: "EXAMPLE0a1b2c3d4e5f60718293a4b5c6"},
			{Plant: "/.env (APP_KEYS)", Value: "EXAMPLE-appkey1==,EXAMPLE-appkey2=="},
		},
	},
	"remix": {
		Name: "remix",
		Paths: []Endpoint{
			{"/.env", "Remix env/secret file probe"},
			{"/remix.config.js", "Remix config source probe"},
			{"/build/index.js", "Remix server-build probe"},
		},
		Prefixes: []Endpoint{
			{"/build/", "Remix build-output walk"},
		},
		Seeds: []CanarySeed{
			{Plant: "/.env (SESSION_SECRET)", Value: "EXAMPLE-remix-session-0a1b2c3d4e5f6071"},
		},
	},
	"sveltekit": {
		Name: "sveltekit",
		Paths: []Endpoint{
			{"/.env", "SvelteKit env/secret file probe"},
			{"/svelte.config.js", "SvelteKit config source probe"},
			{"/vite.config.js", "Vite build config probe"},
		},
		Prefixes: []Endpoint{
			{"/.svelte-kit/", "SvelteKit build-cache walk"},
		},
		Seeds: []CanarySeed{
			{Plant: "/.env (SECRET_API_KEY)", Value: "EXAMPLE-svelte-apikey-8293a4b5c6d7"},
		},
	},
	"nuxt": {
		Name: "nuxt",
		Paths: []Endpoint{
			{"/.env", "Nuxt env/secret file probe"},
			{"/nuxt.config.ts", "Nuxt config source probe"},
			{"/.output/server/index.mjs", "Nuxt server-output probe"},
		},
		Prefixes: []Endpoint{
			{"/.nuxt/", "Nuxt build-cache walk"},
		},
		Seeds: []CanarySeed{
			{Plant: "/.env (NUXT_API_SECRET)", Value: "EXAMPLE-nuxt-secret-0a1b2c3d4e5f6071"},
		},
	},
	"astro": {
		Name: "astro",
		Paths: []Endpoint{
			{"/.env", "Astro env/secret file probe"},
			{"/astro.config.mjs", "Astro config source probe"},
		},
		Prefixes: []Endpoint{
			{"/.astro/", "Astro build-cache walk"},
		},
		Seeds: []CanarySeed{
			{Plant: "/.env (SECRET_TOKEN)", Value: "EXAMPLE-astro-token-8293a4b5c6d7e8f9"},
		},
	},
	"adonisjs": {
		Name: "adonisjs",
		Paths: []Endpoint{
			{"/.env", "AdonisJS env/secret file probe"},
			{"/.adonisrc.json", "AdonisJS project manifest probe"},
			{"/config/database.ts", "AdonisJS database credential probe"},
			{"/ace", "AdonisJS console entrypoint probe"},
		},
		Seeds: []CanarySeed{
			{Plant: "/.env (APP_KEY)", Value: "EXAMPLE-adonis-appkey-0a1b2c3d4e5f6071"},
			{Plant: "/.env (DB_PASSWORD)", Value: "EXAMPLE_adonis_db_pw_7f3a9c21"},
		},
	},
	"meteor": {
		Name: "meteor",
		Paths: []Endpoint{
			{"/settings.json", "Meteor settings/secret probe"},
			{"/.meteor/release", "Meteor version-fingerprint probe"},
		},
		Prefixes: []Endpoint{
			{"/.meteor/local/", "Meteor local build/db directory walk"},
			{"/.meteor/", "Meteor metadata walk"},
		},
		Seeds: []CanarySeed{
			{Plant: "/settings.json (private.apiKey)", Value: "EXAMPLE-meteor-apikey-8293a4b5c6d7"},
		},
	},
	"quarkus": {
		Name: "quarkus",
		Paths: []Endpoint{
			{"/application.properties", "Quarkus config probe"},
			{"/q/dev/", "Quarkus Dev UI probe"},
			{"/q/openapi", "Quarkus OpenAPI enumeration probe"},
			{"/q/health", "Quarkus health-endpoint probe"},
		},
		Prefixes: []Endpoint{
			{"/q/", "Quarkus management-endpoint enumeration"},
		},
		Seeds: []CanarySeed{
			{Plant: "/application.properties (quarkus.datasource.password)", Value: "EXAMPLE_quarkus_pw_3a9c21e4"},
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
