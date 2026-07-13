package output

import (
	"fmt"
	"math"

	"github.com/go-echarts/go-echarts/v2/opts"
)

func coordinates(data opts.GeoData) (longitude, latitude float64, err error) {
	coords, ok := data.Value.([]float64)
	if !ok || len(coords) != 2 {
		return 0, 0, fmt.Errorf("invalid GPS coordinates for %q", data.Name)
	}

	longitude, latitude = coords[0], coords[1]
	if math.IsNaN(latitude) || math.IsInf(latitude, 0) || latitude < -90 || latitude > 90 ||
		math.IsNaN(longitude) || math.IsInf(longitude, 0) || longitude < -180 || longitude > 180 {
		return 0, 0, fmt.Errorf("GPS coordinates for %q are out of range", data.Name)
	}
	return longitude, latitude, nil
}
