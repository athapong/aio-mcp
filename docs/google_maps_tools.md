# Google Maps Tools for MCP

This document describes the Google Maps integration tools available in the Model Context Protocol (MCP) server.

## Setup

1. Obtain a Google Maps API key from the [Google Cloud Console](https://console.cloud.google.com/google/maps-apis)
2. Set the API key as an environment variable:
   ```
   export GOOGLE_MAPS_API_KEY=your_api_key_here
   ```
3. Enable the Google Maps tools using the tool_manager:
   ```
   maps_location_search
   maps_geocoding
   maps_place_details
   ```

## Available Tools

### 1. Location Search (`maps_location_search`)

Search for places and locations using Google Maps.

**Parameters:**
- `query` (string, required): Location or place to search for
- `limit` (integer, optional): Maximum number of results to return (default: 5)

**Example:**
```json
{
  "query": "coffee shops in San Francisco"
}
```

### 2. Geocoding (`maps_geocoding`)

Convert addresses to geographic coordinates (geocoding) or coordinates to addresses (reverse geocoding).

**Parameters for Geocoding:**
- `address` (string): Address to geocode

**Parameters for Reverse Geocoding:**
- `lat` (float): Latitude coordinate
- `lng` (float): Longitude coordinate

**Example - Geocoding:**
```json
{
  "address": "1600 Amphitheatre Parkway, Mountain View, CA"
}
```

**Example - Reverse Geocoding:**
```json
{
  "lat": 37.4224764,
  "lng": -122.0842499
}
```

### 3. Place Details (`maps_place_details`)

Get detailed information about a specific place using its place_id.

**Parameters:**
- `place_id` (string, required): Google Maps place ID

**Example:**
```json
{
  "place_id": "ChIJN1t_tDeuEmsRUsoyG83frY4"
}
```

## Error Handling

All tools will return proper error messages if:
- Required parameters are missing
- The Google Maps API key is not set
- There's an error from the Google Maps API service
