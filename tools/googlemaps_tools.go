package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"

	"github.com/athapong/aio-mcp/util"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"googlemaps.github.io/maps"
)

// RegisterGoogleMapTools registers all Google Maps related tools with the MCP server
func RegisterGoogleMapTools(s *server.MCPServer) {
	// Location search tool
	locationSearchTool := mcp.NewTool("maps_location_search",
		mcp.WithDescription("Search for locations using Google Maps"),
		mcp.WithString("query", mcp.Required(), mcp.Description("Location to search for")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of results to return (default: 5)")),
	)
	s.AddTool(locationSearchTool, util.ErrorGuard(util.AdaptLegacyHandler(locationSearchHandler)))

	// Geocoding tool
	geocodingTool := mcp.NewTool("maps_geocoding",
		mcp.WithDescription("Convert addresses to coordinates and vice versa"),
		mcp.WithString("address", mcp.Description("Address to geocode (required if not using lat/lng)")),
		mcp.WithNumber("lat", mcp.Description("Latitude for reverse geocoding (required with lng if not using address)")),
		mcp.WithNumber("lng", mcp.Description("Longitude for reverse geocoding (required with lat if not using address)")),
	)
	s.AddTool(geocodingTool, util.ErrorGuard(util.AdaptLegacyHandler(geocodingHandler)))

	// Place details tool
	placeDetailsTool := mcp.NewTool("maps_place_details",
		mcp.WithDescription("Get detailed information about a specific place"),
		mcp.WithString("place_id", mcp.Required(), mcp.Description("Google Maps place ID")),
	)
	s.AddTool(placeDetailsTool, util.ErrorGuard(util.AdaptLegacyHandler(placeDetailsHandler)))

	// Directions tool
	directionsTool := mcp.NewTool("maps_directions",
		mcp.WithDescription("Get directions between locations"),
		mcp.WithString("origin", mcp.Required(), mcp.Description("Starting point (address, place ID, or lat,lng)")),
		mcp.WithString("destination", mcp.Required(), mcp.Description("Destination point (address, place ID, or lat,lng)")),
		mcp.WithString("mode", mcp.Description("Travel mode: driving (default), walking, bicycling, transit")),
		mcp.WithString("waypoints", mcp.Description("Optional waypoints separated by '|' (e.g. 'place_id:ChIJ...|place_id:ChIJ...')")),
		mcp.WithBoolean("alternatives", mcp.Description("Return alternative routes if available")),
	)
	s.AddTool(directionsTool, util.ErrorGuard(util.AdaptLegacyHandler(directionsHandler)))
}

// getGoogleMapsClient creates and returns a Google Maps client
func getGoogleMapsClient() (*maps.Client, error) {
	apiKey := os.Getenv("GOOGLE_MAPS_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GOOGLE_MAPS_API_KEY environment variable not set")
	}

	return maps.NewClient(maps.WithAPIKey(apiKey))
}

// locationSearchHandler handles location search requests
func locationSearchHandler(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	query, ok := arguments["query"].(string)
	if !ok || query == "" {
		return mcp.NewToolResultError("query is required and must be a string"), nil
	}

	limit := 5 // default limit
	if limitVal, ok := arguments["limit"].(float64); ok {
		limit = int(limitVal)
	}

	client, err := getGoogleMapsClient()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := &maps.TextSearchRequest{
		Query: query,
	}

	resp, err := client.TextSearch(context.Background(), req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Google Maps API error: %v", err)), nil
	}

	if len(resp.Results) == 0 {
		return mcp.NewToolResultText("No locations found for query: " + query), nil
	}

	// Limit the number of results
	if len(resp.Results) > limit {
		resp.Results = resp.Results[:limit]
	}

	var results []map[string]interface{}
	for _, place := range resp.Results {
		results = append(results, map[string]interface{}{
			"name":     place.Name,
			"address":  place.FormattedAddress,
			"place_id": place.PlaceID,
			"location": map[string]float64{"lat": place.Geometry.Location.Lat, "lng": place.Geometry.Location.Lng},
			"rating":   place.Rating,
			"types":    place.Types,
		})
	}

	data := map[string]interface{}{
		"query":   query,
		"results": results,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal JSON: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

// geocodingHandler handles geocoding and reverse geocoding requests
func geocodingHandler(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	client, err := getGoogleMapsClient()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Check if we're doing geocoding (address to coordinates)
	if address, ok := arguments["address"].(string); ok && address != "" {
		return handleGeocoding(client, address)
	}

	// Check if we're doing reverse geocoding (coordinates to address)
	lat, latOk := arguments["lat"].(float64)
	lng, lngOk := arguments["lng"].(float64)
	if latOk && lngOk {
		return handleReverseGeocoding(client, lat, lng)
	}

	return mcp.NewToolResultError("Please provide either an address for geocoding or lat/lng for reverse geocoding"), nil
}

// handleGeocoding processes an address to get coordinates
func handleGeocoding(client *maps.Client, address string) (*mcp.CallToolResult, error) {
	req := &maps.GeocodingRequest{
		Address: address,
	}

	resp, err := client.Geocode(context.Background(), req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Google Maps API error: %v", err)), nil
	}

	if len(resp) == 0 {
		return mcp.NewToolResultText("No geocoding results found for address: " + address), nil
	}

	var results []map[string]interface{}
	for _, result := range resp {
		results = append(results, map[string]interface{}{
			"formatted_address": result.FormattedAddress,
			"place_id":          result.PlaceID,
			"location":          map[string]float64{"lat": result.Geometry.Location.Lat, "lng": result.Geometry.Location.Lng},
			"location_type":     result.Geometry.LocationType,
			"types":             result.Types,
		})
	}

	data := map[string]interface{}{
		"query":   address,
		"type":    "geocoding",
		"results": results,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal JSON: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

// handleReverseGeocoding processes coordinates to get an address
func handleReverseGeocoding(client *maps.Client, lat, lng float64) (*mcp.CallToolResult, error) {
	req := &maps.GeocodingRequest{
		LatLng: &maps.LatLng{Lat: lat, Lng: lng},
	}

	resp, err := client.Geocode(context.Background(), req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Google Maps API error: %v", err)), nil
	}

	if len(resp) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No reverse geocoding results found for coordinates: %f,%f", lat, lng)), nil
	}

	var results []map[string]interface{}
	for _, result := range resp {
		results = append(results, map[string]interface{}{
			"formatted_address": result.FormattedAddress,
			"place_id":          result.PlaceID,
			"types":             result.Types,
		})
	}

	data := map[string]interface{}{
		"coordinates": map[string]float64{"lat": lat, "lng": lng},
		"type":        "reverse_geocoding",
		"results":     results,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal JSON: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

// placeDetailsHandler handles requests for detailed place information
func placeDetailsHandler(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	placeID, ok := arguments["place_id"].(string)
	if !ok || placeID == "" {
		return mcp.NewToolResultError("place_id is required and must be a string"), nil
	}

	client, err := getGoogleMapsClient()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := &maps.PlaceDetailsRequest{
		PlaceID: placeID,
		Fields: []maps.PlaceDetailsFieldMask{
			maps.PlaceDetailsFieldMaskName,
			maps.PlaceDetailsFieldMaskFormattedAddress,
			maps.PlaceDetailsFieldMaskGeometry,
			maps.PlaceDetailsFieldMaskTypes,
			maps.PlaceDetailsFieldMaskOpeningHours,
			maps.PlaceDetailsFieldMaskWebsite,
			maps.PlaceDetailsFieldMaskReviews,
			maps.PlaceDetailsFieldMaskPhotos,
		},
	}

	resp, err := client.PlaceDetails(context.Background(), req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Google Maps API error: %v", err)), nil
	}

	details := map[string]interface{}{
		"name":               resp.Name,
		"formatted_address":  resp.FormattedAddress,
		"place_id":           resp.PlaceID,
		"location":           map[string]float64{"lat": resp.Geometry.Location.Lat, "lng": resp.Geometry.Location.Lng},
		"types":              resp.Types,
		"rating":             resp.Rating,
		"user_ratings_total": resp.UserRatingsTotal,
	}

	if resp.Website != "" {
		details["website"] = resp.Website
	}

	if resp.FormattedPhoneNumber != "" {
		details["phone_number"] = resp.FormattedPhoneNumber
	}

	if len(resp.OpeningHours.WeekdayText) > 0 {
		details["opening_hours"] = resp.OpeningHours.WeekdayText
	}

	jsonData, err := json.Marshal(details)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal JSON: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

// directionsHandler handles requests for directions between two locations
func directionsHandler(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// Extract required parameters
	origin, ok := arguments["origin"].(string)
	if !ok || origin == "" {
		return mcp.NewToolResultError("origin is required and must be a string"), nil
	}

	destination, ok := arguments["destination"].(string)
	if !ok || destination == "" {
		return mcp.NewToolResultError("destination is required and must be a string"), nil
	}

	// Extract optional parameters
	mode := "driving" // default mode
	if modeVal, ok := arguments["mode"].(string); ok && modeVal != "" {
		switch modeVal {
		case "driving", "walking", "bicycling", "transit":
			mode = modeVal
		default:
			return mcp.NewToolResultError("Invalid mode. Must be one of: driving, walking, bicycling, transit"), nil
		}
	}

	// Create Google Maps client
	client, err := getGoogleMapsClient()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Build directions request
	req := &maps.DirectionsRequest{
		Origin:        origin,
		Destination:   destination,
		Mode:          maps.TravelModeDriving,
		DepartureTime: "now",
	}

	// Add waypoints if provided
	if waypoints, ok := arguments["waypoints"].(string); ok && waypoints != "" {
		req.Waypoints = []string{waypoints}
	}

	// Add alternatives if requested
	if alternatives, ok := arguments["alternatives"].(bool); ok {
		req.Alternatives = alternatives
	}

	// Call the Directions API
	routes, _, err := client.Directions(context.Background(), req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Google Maps API error: %v", err)), nil
	}

	if len(routes) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No directions found from %s to %s", origin, destination)), nil
	}

	// Format the response
	var formattedRoutes []map[string]interface{}
	for i, route := range routes {
		routeInfo := map[string]interface{}{
			"summary": route.Summary,
		}

		// Calculate total distance and duration
		var totalDistance int
		var totalDuration float64
		var steps []map[string]interface{}

		for _, leg := range route.Legs {
			totalDistance += leg.Distance.Meters
			totalDuration += leg.Duration.Seconds()

			for _, step := range leg.Steps {
				stepInfo := map[string]interface{}{
					"instruction":      step.HTMLInstructions,
					"distance":         map[string]interface{}{"meters": step.Distance.Meters, "text": step.Distance.HumanReadable},
					"duration":         map[string]interface{}{"seconds": step.Duration.Seconds(), "text": step.Duration.String()},
					"travel_mode":      step.TravelMode,
					"start_location":   map[string]float64{"lat": step.StartLocation.Lat, "lng": step.StartLocation.Lng},
					"end_location":     map[string]float64{"lat": step.EndLocation.Lat, "lng": step.EndLocation.Lng},
					"encoded_polyline": step.Polyline.Points,
				}
				steps = append(steps, stepInfo)
			}
		}
		// Format as hours and minutes for better readability
		hours := int(totalDuration / 3600)
		minutes := int(math.Mod(totalDuration, 3600) / 60)
		durationText := ""
		durationText = ""
		if hours > 0 {
			durationText = fmt.Sprintf("%d hours", hours)
			if minutes > 0 {
				durationText += fmt.Sprintf(" %d minutes", minutes)
			}
		} else {
			durationText = fmt.Sprintf("%d minutes", minutes)
		}

		// Add distance and duration info
		routeInfo["distance"] = map[string]interface{}{
			"meters": totalDistance,
			"text":   fmt.Sprintf("%.1f km", float64(totalDistance)/1000),
		}
		routeInfo["duration"] = map[string]interface{}{
			"seconds": totalDuration,
			"text":    durationText,
		}
		routeInfo["steps"] = steps
		routeInfo["encoded_overview_polyline"] = route.OverviewPolyline.Points
		routeInfo["warnings"] = route.Warnings
		routeInfo["route_index"] = i

		formattedRoutes = append(formattedRoutes, routeInfo)
	}

	data := map[string]interface{}{
		"origin":      origin,
		"destination": destination,
		"mode":        mode,
		"routes":      formattedRoutes,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal JSON: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
