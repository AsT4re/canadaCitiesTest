package main

import(
	"math"
)

func radians(deg float64) float64 {
	return deg * math.Pi / 180
}

func degrees(rad float64) float64 {
	return rad * 180 / math.Pi
}

var (
	EARTH_RADIUS = 6371.01
	MIN_LAT = radians(-90)
	MAX_LAT = radians(90)
	MIN_LON = radians(-180)
	MAX_LON = radians(180)
)

func getBoundingBox(lon, lat, dist float64) (float64, float64, float64, float64) {
	angular_distance := dist/EARTH_RADIUS

	lon_rad := radians(lon)
	lat_rad := radians(lat)

	min_lat := lat_rad - angular_distance
	max_lat := lat_rad + angular_distance

	var min_lon, max_lon float64

	if min_lat > MIN_LAT && max_lat < MAX_LAT {
		delta_lon := math.Asin(math.Sin(angular_distance) / math.Cos(lat_rad))

		min_lon = lon_rad - delta_lon
		if min_lon < MIN_LON {
			min_lon += 2 * math.Pi
		}

		max_lon = lon_rad + delta_lon
		if max_lon > MAX_LON {
			max_lon -= 2 * math.Pi
		}
	} else {
		min_lat = math.Max(min_lat, MIN_LAT)
		max_lat = math.Min(max_lat, MAX_LAT)
		min_lon = MIN_LON
		max_lon = MAX_LON
	}

	min_lon_deg := degrees(min_lon)
	min_lat_deg := degrees(min_lat)
	max_lon_deg := degrees(max_lon)
	max_lat_deg := degrees(max_lat)

	return min_lat_deg, min_lon_deg, max_lat_deg, max_lon_deg
}
