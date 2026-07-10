// Package output provides functions to generate output formats like HTML maps or GPX files.
package output

import (
	"fmt"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
)

// GenerateMap creates an HTML file with a world map and pins based on GPS coordinates extracted from images.
// The map is saved to "map.html".
func GenerateMap(gpsData []opts.GeoData) error {
	geo := charts.NewGeo()
	geo.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{Title: "photos2map: GPS Image Map"}),
		charts.WithGeoComponentOpts(opts.GeoComponent{
			// map comes from https://github.com/echarts-maps/echarts-countries-js/tree/master/echarts-countries-js
			Map:       "USA",
			ItemStyle: &opts.ItemStyle{Color: "#006666"},
		}),
	)

	geo.AddSeries("geo", types.ChartEffectScatter, gpsData,
		charts.WithRippleEffectOpts(opts.RippleEffect{
			Period:    4,
			Scale:     6,
			BrushType: "stroke",
		}),
	)

	file, commit, err := createAtomic("out/map.html")
	if err != nil {
		return fmt.Errorf("create map output: %w", err)
	}
	defer file.abort()

	if err := geo.Render(file.File); err != nil {
		return fmt.Errorf("render map output: %w", err)
	}
	if err := commit(); err != nil {
		return fmt.Errorf("commit map output: %w", err)
	}
	return nil
}
