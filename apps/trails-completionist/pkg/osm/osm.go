package osm

import (
	"encoding/gob"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
)

// OSM XML data structures
type OSMData struct {
	Nodes map[int64]OSMNode
	Ways  map[int64]OSMWay
}

type OSMNode struct {
	ID   int64
	Lat  float64
	Lon  float64
	Tags map[string]string
}

type OSMWay struct {
	ID    int64
	Nodes []int64
	Tags  map[string]string
	BBox  [4]float64 // min_lat, min_lon, max_lat, max_lon
}

// XML parsing structures
type OSM struct {
	XMLName xml.Name `xml:"osm"`
	Nodes   []Node   `xml:"node"`
	Ways    []Way    `xml:"way"`
}

type Node struct {
	ID   string  `xml:"id,attr"`
	Lat  float64 `xml:"lat,attr"`
	Lon  float64 `xml:"lon,attr"`
	Tags []Tag   `xml:"tag"`
}

type Way struct {
	ID       string    `xml:"id,attr"`
	NodeRefs []NodeRef `xml:"nd"`
	Tags     []Tag     `xml:"tag"`
}

type NodeRef struct {
	Ref string `xml:"ref,attr"`
}

type Tag struct {
	Key   string `xml:"k,attr"`
	Value string `xml:"v,attr"`
}

// Trail represents a trail from the OSM data
type Trail struct {
	ID     int64
	Name   string
	Type   string
	WayIDs []int64
}

// loadOSMData loads OSM data, first checking for a cached binary version
func LoadOSMData(osmFilePath string, forceReload bool) (*OSMData, error) {
	// Define binary cache file path based on the OSM file path
	binaryPath := osmFilePath + ".bin"

	// Check if we can use the cached binary version
	if !forceReload {
		osmData, err := tryLoadBinary(osmFilePath, binaryPath)
		if err == nil {
			fmt.Println("Loaded OSM data from binary cache.")
			return osmData, nil
		}
		fmt.Printf("Could not use binary cache: %v\n", err)
	}

	// If binary loading fails or is forced to reload, load from XML
	fmt.Println("Parsing OSM XML file...")
	osmData, err := loadOSMFile(osmFilePath)
	if err != nil {
		return nil, err
	}

	// Save to binary for future use
	fmt.Println("Saving parsed data to binary cache...")
	err = saveToBinary(osmData, binaryPath)
	if err != nil {
		fmt.Printf("Warning: Failed to save binary cache: %v\n", err)
	}

	return osmData, nil
}

// tryLoadBinary attempts to load OSM data from the binary cache file
func tryLoadBinary(osmFilePath, binaryPath string) (*OSMData, error) {
	// Check if binary file exists
	binaryInfo, err := os.Stat(binaryPath)
	if err != nil {
		return nil, fmt.Errorf("binary cache doesn't exist")
	}

	// Check if XML file exists and get its modification time
	xmlInfo, err := os.Stat(osmFilePath)
	if err != nil {
		return nil, fmt.Errorf("can't access original XML file: %w", err)
	}

	// If XML is newer than binary, binary is outdated
	if xmlInfo.ModTime().After(binaryInfo.ModTime()) {
		return nil, fmt.Errorf("binary cache is outdated")
	}

	// Load the binary file
	fmt.Println("Loading OSM map data from binary cache...")
	file, err := os.Open(binaryPath) // #nosec G304
	if err != nil {
		return nil, fmt.Errorf("error opening binary file: %w", err)
	}
	defer file.Close()

	// Decode the gob data
	decoder := gob.NewDecoder(file)
	var osmData OSMData
	err = decoder.Decode(&osmData)
	if err != nil {
		return nil, fmt.Errorf("error decoding binary data: %w", err)
	}

	return &osmData, nil
}

// saveToBinary saves the OSM data to a binary file using gob encoding
func saveToBinary(osmData *OSMData, binaryPath string) error {
	file, err := os.Create(binaryPath) // #nosec G304
	if err != nil {
		return fmt.Errorf("error creating binary file: %w", err)
	}
	defer file.Close()

	// Encode the data
	encoder := gob.NewEncoder(file)
	err = encoder.Encode(osmData)
	if err != nil {
		return fmt.Errorf("error encoding data: %w", err)
	}

	return nil
}

// loadOSMFile loads and parses an OSM XML file
func loadOSMFile(filePath string) (*OSMData, error) {
	file, err := os.Open(filePath) // #nosec G304
	if err != nil {
		return nil, fmt.Errorf("error opening OSM file: %w", err)
	}
	defer file.Close()

	osmData := &OSMData{
		Nodes: make(map[int64]OSMNode),
		Ways:  make(map[int64]OSMWay),
	}

	decoder := xml.NewDecoder(file)

	// Temporary variables to store current node/way being processed
	var currentNode OSMNode
	var currentWay OSMWay
	var inNode, inWay bool

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error parsing XML: %w", err)
		}

		switch se := token.(type) {
		case xml.StartElement:
			switch se.Name.Local {
			case "node":
				inNode = true
				inWay = false
				currentNode = OSMNode{Tags: make(map[string]string)}

				// Parse node attributes
				for _, attr := range se.Attr {
					switch attr.Name.Local {
					case "id":
						if id, err := strconv.ParseInt(attr.Value, 10, 64); err == nil {
							currentNode.ID = id
						}
					case "lat":
						if lat, err := strconv.ParseFloat(attr.Value, 64); err == nil {
							currentNode.Lat = lat
						}
					case "lon":
						if lon, err := strconv.ParseFloat(attr.Value, 64); err == nil {
							currentNode.Lon = lon
						}
					}
				}

			case "way":
				inNode = false
				inWay = true
				currentWay = OSMWay{
					Nodes: []int64{},
					Tags:  make(map[string]string),
				}

				// Parse way attributes
				for _, attr := range se.Attr {
					if attr.Name.Local == "id" {
						if id, err := strconv.ParseInt(attr.Value, 10, 64); err == nil {
							currentWay.ID = id
						}
					}
				}

			case "nd":
				if inWay {
					for _, attr := range se.Attr {
						if attr.Name.Local == "ref" {
							if ref, err := strconv.ParseInt(attr.Value, 10, 64); err == nil {
								currentWay.Nodes = append(currentWay.Nodes, ref)
							}
						}
					}
				}

			case "tag":
				var key, value string
				for _, attr := range se.Attr {
					if attr.Name.Local == "k" {
						key = attr.Value
					} else if attr.Name.Local == "v" {
						value = attr.Value
					}
				}

				if key != "" {
					if inNode {
						currentNode.Tags[key] = value
					} else if inWay {
						currentWay.Tags[key] = value
					}
				}
			}

		case xml.EndElement:
			switch se.Name.Local {
			case "node":
				osmData.Nodes[currentNode.ID] = currentNode
				inNode = false

			case "way":
				// Calculate bounding box for the way
				if len(currentWay.Nodes) > 0 {
					currentWay.BBox = CalculateWayBBox(currentWay, osmData.Nodes)
				}
				osmData.Ways[currentWay.ID] = currentWay
				inWay = false
			}
		}
	}

	return osmData, nil
}

// calculateWayBBox calculates the bounding box for a way based on its nodes
func CalculateWayBBox(way OSMWay, nodes map[int64]OSMNode) [4]float64 {
	if len(way.Nodes) == 0 {
		return [4]float64{0, 0, 0, 0}
	}

	minLat, maxLat := 90.0, -90.0
	minLon, maxLon := 180.0, -180.0

	for _, nodeID := range way.Nodes {
		if node, exists := nodes[nodeID]; exists {
			minLat = math.Min(minLat, node.Lat)
			maxLat = math.Max(maxLat, node.Lat)
			minLon = math.Min(minLon, node.Lon)
			maxLon = math.Max(maxLon, node.Lon)
		}
	}

	return [4]float64{minLat, minLon, maxLat, maxLon}
}

// overlaps checks if two bounding boxes overlap
func Overlaps(box1, box2 [4]float64) bool {
	// box format: [minLat, minLon, maxLat, maxLon]
	return box1[0] <= box2[2] && box1[2] >= box2[0] && // Latitude overlap
		box1[1] <= box2[3] && box1[3] >= box2[1] // Longitude overlap
}
