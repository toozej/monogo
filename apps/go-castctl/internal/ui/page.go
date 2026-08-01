// Package ui defines the go-app browser interface shared by prerendering and WASM.
package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

var registerOnce sync.Once

// RegisterRoutes installs go-app routes exactly once for native and WASM builds.
func RegisterRoutes() {
	registerOnce.Do(func() {
		app.Route("/", func() app.Composer { return &Page{DeviceType: "chromecast"} })
	})
}

type device struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Address string `json:"address"`
}

type request struct {
	URL    string `json:"url"`
	Device device `json:"device"`
}

type response struct {
	Status    string   `json:"status"`
	Message   string   `json:"message"`
	StreamURL string   `json:"streamURL"`
	BitRate   int64    `json:"bitrate"`
	Devices   []device `json:"devices"`
}

// Page is the go-castctl single-page interface.
type Page struct {
	app.Compo

	URL           string
	DeviceName    string
	DeviceType    string
	DeviceAddress string
	Devices       []device
	StreamURL     string
	Message       string
	Error         string
	Busy          bool
	Scanning      bool
}

// OnMount discovers nearby devices without blocking initial rendering.
func (p *Page) OnMount(ctx app.Context) {
	p.discover(ctx)
}

// Render builds the responsive casting console and local player.
func (p *Page) Render() app.UI {
	deviceButtons := make([]app.UI, 0, len(p.Devices))
	for i := range p.Devices {
		found := p.Devices[i]
		deviceButtons = append(deviceButtons,
			app.Button().
				Type("button").
				Class("device").
				OnClick(func(ctx app.Context, _ app.Event) {
					p.DeviceName = found.Name
					p.DeviceType = found.Type
					p.DeviceAddress = found.Address
				}).
				Body(
					app.Span().Class("device-name").Text(found.Name),
					app.Small().Text(fmt.Sprintf("%s · %s", found.Type, found.Address)),
				),
		)
	}

	return app.Div().Body(
		app.Style().Text(styles),
		app.Main().Class("shell").Body(
			app.Header().Class("hero").Body(
				app.Div().Class("mark").Text("▶"),
				app.Div().Body(
					app.H1().Text("go-castctl"),
					app.P().Text("Send web video to the big screen—or bring the stream back here."),
				),
			),
			app.Section().Class("panel").Body(
				app.Label().For("source-url").Text("Video page URL"),
				app.Input().ID("source-url").Type("url").Required(true).
					Placeholder("https://example.com/watch/video").
					Value(p.URL).OnInput(p.ValueTo(&p.URL)),
				app.Div().Class("section-heading").Body(
					app.H2().Text("TV endpoint"),
					app.Button().Type("button").Class("ghost").Disabled(p.Scanning).
						OnClick(func(ctx app.Context, _ app.Event) { p.discover(ctx) }).
						Textf("%s", map[bool]string{true: "Scanning…", false: "Scan TVs"}[p.Scanning]),
				),
				app.If(len(deviceButtons) != 0, func() app.UI {
					return app.Div().Class("devices").Body(deviceButtons...)
				}),
				app.Div().Class("grid").Body(
					app.Div().Body(
						app.Label().For("device-name").Text("Name"),
						app.Input().ID("device-name").Placeholder("Living Room TV").
							Value(p.DeviceName).OnInput(p.ValueTo(&p.DeviceName)),
					),
					app.Div().Body(
						app.Label().For("device-type").Text("Type"),
						app.Select().ID("device-type").OnChange(p.ValueTo(&p.DeviceType)).Body(
							app.Option().Value("chromecast").Selected(p.DeviceType == "chromecast").Text("Chromecast"),
							app.Option().Value("dlna").Selected(p.DeviceType == "dlna").Text("DLNA"),
						),
					),
				),
				app.Label().For("device-address").Text("Discovered address"),
				app.Input().ID("device-address").Placeholder("Shown after selecting a scanned TV").
					Value(p.DeviceAddress).Disabled(true),
				app.Div().Class("actions").Body(
					app.Button().Type("button").Class("primary").Disabled(p.Busy).
						OnClick(func(ctx app.Context, _ app.Event) { p.submit(ctx, "/send") }).Text("Cast to TV"),
					app.Button().Type("button").Class("secondary").Disabled(p.Busy).
						OnClick(func(ctx app.Context, _ app.Event) { p.submit(ctx, "/receive") }).Text("Watch here"),
				),
				app.If(p.Busy, func() app.UI { return app.P().Class("status").Text("Castor is finding the stream…") }),
				app.If(p.Message != "", func() app.UI { return app.P().Class("success").Text(p.Message) }),
				app.If(p.Error != "", func() app.UI { return app.P().Class("error").Text(p.Error) }),
			),
			app.If(p.StreamURL != "", func() app.UI {
				return app.Section().Class("panel player-panel").Body(
					app.Div().Class("section-heading").Body(
						app.H2().Text("Now watching"),
						app.A().Href(p.StreamURL).Target("_blank").Rel("noreferrer").Text("Open stream ↗"),
					),
					app.Video().ID("local-player").Src(p.StreamURL).Controls(true).AutoPlay(true),
				)
			}),
			app.P().Class("fine-print").Text("Use only with video you are authorized to view. DRM-protected streams are not supported."),
		),
	)
}

func (p *Page) discover(ctx app.Context) {
	if p.Scanning {
		return
	}
	p.Scanning = true
	p.Error = ""
	ctx.Async(func() {
		res, err := http.Get("/devices")
		var decoded response
		if err == nil {
			defer func() { _ = res.Body.Close() }()
			err = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&decoded)
			if err == nil && res.StatusCode != http.StatusOK {
				err = fmt.Errorf("%s", decoded.Message)
			}
		}
		ctx.Dispatch(func(ctx app.Context) {
			p.Scanning = false
			if err != nil {
				p.Error = "TV scan failed: " + err.Error()
				return
			}
			p.Devices = decoded.Devices
			if len(p.Devices) == 1 {
				p.DeviceName = p.Devices[0].Name
				p.DeviceType = p.Devices[0].Type
				p.DeviceAddress = p.Devices[0].Address
			}
		})
	})
}

func (p *Page) submit(ctx app.Context, endpoint string) {
	if p.Busy {
		return
	}
	p.Busy = true
	p.Error = ""
	p.Message = ""
	payload, _ := json.Marshal(request{
		URL: p.URL,
		Device: device{
			Name: p.DeviceName, Type: p.DeviceType, Address: p.DeviceAddress,
		},
	})
	ctx.Async(func() {
		// #nosec G107 -- endpoint is selected internally from /send and /receive, never from user input.
		res, err := http.Post(endpoint, "application/json", bytes.NewReader(payload))
		var decoded response
		if err == nil {
			defer func() { _ = res.Body.Close() }()
			err = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&decoded)
			if err == nil && res.StatusCode >= http.StatusBadRequest {
				err = fmt.Errorf("%s", decoded.Message)
			}
		}
		ctx.Dispatch(func(ctx app.Context) {
			p.Busy = false
			if err != nil {
				p.Error = err.Error()
				return
			}
			p.StreamURL = decoded.StreamURL
			if decoded.Message != "" {
				p.Message = decoded.Message
			} else if decoded.StreamURL != "" {
				p.Message = "Stream resolved for local playback."
			}
		})
	})
}

const styles = `
:root { color-scheme: dark; font-family: Inter, ui-sans-serif, system-ui, sans-serif; background: #090d18; color: #f5f7ff; }
* { box-sizing: border-box; }
body { margin: 0; min-height: 100vh; background: radial-gradient(circle at 15% 0%, #292054 0, transparent 35rem), #090d18; }
button, input, select { font: inherit; }
.shell { width: min(880px, calc(100% - 2rem)); margin: 0 auto; padding: 4rem 0 3rem; }
.hero { display: flex; align-items: center; gap: 1rem; margin-bottom: 1.5rem; }
.hero h1 { margin: 0; font-size: clamp(2.25rem, 8vw, 4rem); letter-spacing: -.07em; }
.hero p { margin: .35rem 0 0; color: #b8bed5; }
.mark { width: 3.5rem; height: 3.5rem; display: grid; place-items: center; border-radius: 1rem; background: linear-gradient(135deg, #8b7cff, #4bd8db); color: #090d18; box-shadow: 0 1rem 3rem #6d5dfc55; }
.panel { padding: clamp(1.25rem, 4vw, 2rem); border: 1px solid #ffffff18; border-radius: 1.4rem; background: #111728e8; box-shadow: 0 1.5rem 5rem #0007; backdrop-filter: blur(16px); margin-bottom: 1rem; }
label { display: block; color: #cdd2e6; font-size: .82rem; font-weight: 700; letter-spacing: .06em; text-transform: uppercase; margin: .8rem 0 .45rem; }
input, select { width: 100%; border: 1px solid #ffffff24; border-radius: .8rem; color: #fff; background: #080d1a; padding: .85rem 1rem; outline: none; }
input:focus, select:focus { border-color: #8b7cff; box-shadow: 0 0 0 3px #8b7cff22; }
.grid { display: grid; grid-template-columns: 2fr 1fr; gap: 1rem; }
.section-heading { display: flex; justify-content: space-between; align-items: center; gap: 1rem; margin-top: 1.4rem; }
.section-heading h2 { margin: 0; font-size: 1.05rem; }
.section-heading a { color: #9ee7e5; }
.devices { display: grid; grid-template-columns: repeat(auto-fit, minmax(190px, 1fr)); gap: .65rem; margin-top: .8rem; }
.device { text-align: left; border: 1px solid #ffffff18; border-radius: .8rem; background: #171e33; color: #fff; padding: .75rem; cursor: pointer; }
.device:hover { border-color: #8b7cff; transform: translateY(-1px); }
.device span, .device small { display: block; }
.device-name { font-weight: 750; }
.device small { margin-top: .25rem; color: #929ab7; }
.actions { display: flex; gap: .75rem; margin-top: 1.35rem; }
.actions button, .ghost { border: 0; border-radius: .8rem; cursor: pointer; font-weight: 750; padding: .85rem 1.1rem; }
.primary { flex: 1; background: linear-gradient(135deg, #8b7cff, #6d5dfc); color: #fff; }
.secondary { flex: 1; background: #21304a; color: #d8efff; }
.ghost { padding: .5rem .75rem; background: #ffffff12; color: #b9c2e2; }
button:disabled { cursor: wait; opacity: .55; }
.status, .success, .error { border-radius: .75rem; padding: .75rem 1rem; margin: 1rem 0 0; }
.status { background: #6d5dfc1e; color: #c5bdff; }
.success { background: #42d39218; color: #8cf0bb; }
.error { background: #ff5f7a18; color: #ff9cad; }
.player-panel video { width: 100%; min-height: 240px; margin-top: 1rem; border-radius: .9rem; background: #000; }
.fine-print { color: #707995; font-size: .78rem; text-align: center; margin: 1.5rem auto 0; }
@media (max-width: 600px) { .shell { padding-top: 2rem; } .grid { grid-template-columns: 1fr; gap: 0; } .actions { flex-direction: column; } }
`
