module codeberg.org/readeck/readeck

go 1.27.4

require (
	codeberg.org/readeck/go-readability/v2 v2.1.3
	github.com/JohannesKaufmann/html-to-markdown/v2 v2.5.2
	github.com/a-h/templ v0.3.1118
	github.com/andybalholm/cascadia v1.3.5
	github.com/antchfx/htmlquery v1.3.7
	github.com/antchfx/xmlquery v1.5.2
	github.com/anthonynsimon/bild v0.17.1
	github.com/aymerick/douceur v0.2.0
	github.com/caarlos0/env/v11 v11.4.2
	github.com/coreos/go-oidc/v3 v3.21.0
	github.com/cristalhq/acmd v0.12.0
	github.com/dop251/goja v0.0.0-20261110204830-f493a2b600d6
	github.com/dop251/goja_nodejs v0.0.0-20260918173711-b481721df8a2
	github.com/doug-martin/goqu/v9 v9.25.0
	github.com/fxamacker/cbor/v2 v2.10.0
	github.com/gabriel-vasile/mimetype v1.4.16
	github.com/go-chi/chi/v5 v5.5.0
	github.com/go-jose/go-jose/v4 v4.1.9
	github.com/go-shiori/dom v0.0.0-20230515143342-73569d674e1c
	github.com/gobwas/glob v1.1.0
	github.com/google/uuid v1.6.0
	github.com/hlandau/passlib v1.0.11
	github.com/itlightning/dateparse v0.2.1
	github.com/jackc/pgx/v5 v5.13.0
	github.com/jarcoal/httpmock v1.4.2
	github.com/kinbiko/jsonassert v1.2.0
	github.com/klauspost/compress v1.20.4
	github.com/komkom/toml v0.1.2
	github.com/leonelquinteros/gotext v1.8.0
	github.com/mangoumbrella/goldmark-figure v1.4.0
	github.com/mattn/go-sqlite3 v1.14.53
	github.com/phsym/console-slog v0.3.1
	github.com/pmezard/go-difflib v1.0.0
	github.com/prometheus/client_golang v1.25.0
	github.com/redis/go-redis/v9 v9.23.0
	github.com/sergeymakinen/go-ico v1.0.0
	github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e
	github.com/stretchr/testify v1.12.1
	github.com/tdewolff/parse/v2 v2.8.20
	github.com/wneessen/go-mail v0.8.2
	github.com/wyatt915/treeblood v0.1.16
	github.com/yuin/goldmark v1.8.7
	github.com/yuin/goldmark-meta v1.1.0
	golang.org/x/crypto v0.59.0
	golang.org/x/image v0.48.0
	golang.org/x/net v0.62.0
	golang.org/x/oauth2 v0.38.0
	golang.org/x/sync v0.25.0
	golang.org/x/term v0.48.0
	golang.org/x/text v0.44.0
	gopkg.in/yaml.v3 v3.0.1
	modernc.org/sqlite v1.62.0
)

require (
	github.com/JohannesKaufmann/dom v0.3.1 // indirect
	github.com/PuerkitoBio/goquery v1.13.0 // indirect
	github.com/antchfx/xpath v1.3.9 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dlclark/regexp2/v2 v2.13.0 // indirect
	github.com/dustin/go-humanize v1.1.0 // indirect
	github.com/go-sourcemap/sourcemap v2.1.4+incompatible // indirect
	github.com/gogs/chardet v0.0.0-20211120154057-b7413eaefb8f // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/google/pprof v0.0.0-20261111200114-fac431ef4d26 // indirect
	github.com/gorilla/css v1.0.1 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.3 // indirect
	github.com/kr/text v0.1.0 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/ncruces/go-strftime v1.1.0 // indirect
	github.com/pandatix/go-abnf v0.5.5 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_model v0.6.3 // indirect
	github.com/prometheus/common v0.74.0 // indirect
	github.com/prometheus/procfs v0.23.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/rogpeppe/go-internal v1.15.0 // indirect
	github.com/sergeymakinen/go-bmp v1.0.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.uber.org/atomic v1.12.0 // indirect
	go.yaml.in/yaml/v3 v3.0.6 // indirect
	golang.org/x/sys v0.50.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
	gopkg.in/hlandau/easymetric.v1 v1.0.0 // indirect
	gopkg.in/hlandau/measurable.v1 v1.0.1 // indirect
	gopkg.in/hlandau/passlib.v1 v1.0.11 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	modernc.org/libc v1.81.0 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
)

ignore (
	./assets/www
	./web
)
