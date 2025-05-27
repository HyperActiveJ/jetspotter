package jetspotter

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"os"
	
	"jetspotter/internal/configuration"
	"jetspotter/internal/metrics"
	"jetspotter/internal/planespotter"
	"jetspotter/internal/weather"

	"github.com/jftuga/geodist"
	
	"github.com/nathan-fiscaletti/consolesize-go"
)

// Vars
var (
	baseURL = "https://api.adsb.one/v2"
	//baseURL2 = "https://api.airplanes.live/v2"
	baseURL2="https://api.adsb.lol/v2/"
)




// HandleAircraft return a list of aircraft that have been filtered by range, type and altitude.
// Aircraft that have been spotted are removed from the list.
func HandleAircraft(alreadySpottedAircraft *[]Aircraft, config configuration.Config) (aircraft []AircraftOutput, err error) {
	var newlySpottedAircraft []Aircraft
	//Get Aircraft at max range
	if config.RunOnce == 0{
		fmt.Printf("\033[2J")
	}
	
	allAircraftInRange, err := getAllAircraftInRange(baseURL2, config.Location, config.MaxRangeKilometersR)
	if err != nil {
		fmt.Printf("Err %s\r\n",baseURL2)
		config.ADSBX  = 2
		//return nil, err
	}
	
	if  config.ADSBX == 2{
		time.Sleep(30)
	}
	
	if config.ADSBX > 0{
		allAircraftInRange2, err := getAllAircraftInRange(baseURL, config.Location, config.MaxRangeKilometersR)
		if err != nil {
			return nil, err
		}
		if  config.ADSBX < 2{
			for _, ac := range allAircraftInRange {
				add := true
				for _, ac2 := range allAircraftInRange2 {
					if ac.ICAO == ac2.ICAO  {
						add = false
					}
				}
				if add {
					allAircraftInRange2 = append(allAircraftInRange2, ac)
				}
			}
			fmt.Printf("%d %d %d\r\n",len(allAircraftInRange2),len(allAircraftInRange2)-len(allAircraftInRange),len(allAircraftInRange))
			allAircraftInRange = allAircraftInRange2
		}
	} 
	if  config.ADSBX == 2{
		 config.ADSBX = 0
	}
	
	
	for _, ac := range allAircraftInRange {
		aircraftLocation := geodist.Coord{Lat: ac.Lat, Lon: ac.Lon}
		ac.Dst = CalculateDistanceF(config.Location, aircraftLocation)
		ac.Dir = CalculateBearing(config.Location, aircraftLocation)
		ac.Dst2 = CalculateDistanceF(config.Location2, aircraftLocation)
		ac.Dir2 = CalculateBearing(config.Location2, aircraftLocation)
	}
	
	sort.Slice(allAircraftInRange, func(i, j int) bool {
		return allAircraftInRange[i].Dst < allAircraftInRange[j].Dst
	})

	//Generate all metadata that is reused
	//CreateAircraftOutput(allAircraftInRange, config)
	filteredAircraft2 := filterAircraftByAltitude(allAircraftInRange, -1000,config.MaxAltitudeFeet, 0)
	filteredAircraft2 = filterAircraftDistance(filteredAircraft2, 0, float64(config.MaxRangeKilometers), 0, config)
	filteredAircraft3 := filterAircraftByAltitude(allAircraftInRange, config.MaxAltitudeFeet, config.MaxAltitudeFeet2, 0)
	filteredAircraft3 = filterAircraftDistance(filteredAircraft3, 0, float64(config.MaxRangeKilometers2), 0, config)
	if config.RunOnce == 0{
		for _, ac := range filteredAircraft2 {
			fmt.Printf("%5s ", ac.PlaneType)
		}
		fmt.Printf(" - ")
		for _, ac := range filteredAircraft3 {
			fmt.Printf("%5s ", ac.PlaneType)
		}
		fmt.Printf("\r\n")
		for _, ac := range filteredAircraft2 {
			fmt.Printf("%05d ", ac.AltGeom)
		}
		fmt.Printf(" - ")
		for _, ac := range filteredAircraft3 {
			fmt.Printf("%05d ", ac.AltGeom)
		}
		fmt.Printf("\r\n")
		for _, ac := range filteredAircraft2 {
			fmt.Printf("%5.0f ", math.Round(ac.GS))
		}
		fmt.Printf(" - ")
		for _, ac := range filteredAircraft3 {
			fmt.Printf("%5.0f ", math.Round(ac.GS))
		}
		fmt.Printf("\r\n")
		for _, ac := range filteredAircraft2 {
			fmt.Printf("%5.2f ", ac.Dst)
		}
		fmt.Printf(" - ")
		for _, ac := range filteredAircraft3 {
			fmt.Printf("%5.2f ", ac.Dst)
		}
		fmt.Printf("\r\n")
		fmt.Printf("\r\n")
	}

	//Look for never before seen
	filteredAircraft4 := allAircraftInRange
	//filteredAircraft4 := filterAircraftByAltitude(allAircraftInRange, config.MaxAltitudeFeet, config.MaxAltitudeFeet2, config.PrintIgnores)
	filteredAircraft4 = filterAircraftByTypes(filteredAircraft4, config.AircraftTypes, config.AircraftTypesExcl2, config.PrintIgnores, "AircraftTypesExcl2",config)
	filteredAircraft4 = filterAircraftByTypes(filteredAircraft4, config.AircraftTypes, config.AircraftTypesExcl, config.PrintIgnores, "AircraftTypesExcl",config)
	filteredAircraft4 = filterAircraftByTypes(filteredAircraft4, config.AircraftTypes, config.AircraftTypesKnownInteresting, config.PrintIgnores, "AircraftTypesKnownInteresting",config)
	PrintACs(filteredAircraft4, config, "Never Before SEEN!", "!*!*!")
	filteredAircraft4, *alreadySpottedAircraft = validateAircraft(filteredAircraft4, alreadySpottedAircraft)
	LogACs(filteredAircraft4, config, "Never Before SEEN!", "!*!*!")

	//Find Landing And Takign Off b
	filteredAircraft1 := filterAircraftByAltitude(allAircraftInRange, -1000, config.MaxAltitudeFeet2, 0)
	filteredAircraft1 = filterAircraftDistance(filteredAircraft1, 0, float64(config.MaxRangeKilometers2), 0, config)
	if config.PrintApproach > 0{
		landingAircaft(filteredAircraft1, config)
	}
	if config.PrintDeparture > 0{
		departingAircaft(filteredAircraft1, config)
	}
	
	
	//Look for interesting aircraft nearby and low
	if config.PritnStrange > 0{
		PrintStrange(filteredAircraft2, config, "Strange", "STR")
	}
	
	
	filteredAircraft2 = filterAircraftByTypes(filteredAircraft2, config.AircraftTypes, config.AircraftTypesExcl, 0, "Low",config)
	if config.PrintInteresting > 0{
		PrintACs(filteredAircraft2, config, "Low", "***")
	}
	if config.PrintFlag2 > 0{
		PrintFlagged(filteredAircraft2, config, "Flagged", "FLG")
	}
	if config.PrintMil > 0{
		PrintFlagged(filteredAircraft2, config, "MIL", "MIL")
	}
	
	

	
	
	//Look For Interesting Aircraft High and at up to a larger range

	if config.PritnStrange > 0{
		PrintStrange(filteredAircraft3, config, "Strange", "STR")
	}
	filteredAircraft3 = filterAircraftByTypes(filteredAircraft3, config.AircraftTypes, config.AircraftTypesExcl2, 0, "High",config)
	filteredAircraft3 = filterAircraftByTypes(filteredAircraft3, config.AircraftTypes, config.AircraftTypesExcl, 0, "High",config)
	if config.PrintInteresting > 0{
		PrintACs(filteredAircraft3, config, "High", "!!!")
	}
	if config.PrintFlag2 > 0{
		PrintFlagged(filteredAircraft3, config, "Flagged", "FLG")
	}
	if config.PrintMil > 0{
		PrintFlagged(filteredAircraft3, config, "MIL", "MIL")
	}


	
	//Optionally print boring aircarft
	if(config.PrintIgnores > 0){
		PrintACs(newlySpottedAircraft, config, "New", "")
	}
	

	fmt.Printf("\r\n")
	
	//Done
	return nil, nil
}

func LogACs(allFilteredAircraft []Aircraft, config configuration.Config, filters string, marks string)  {
	for _, ac := range allFilteredAircraft {
		f, err := os.OpenFile("Interesting.txt", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			panic(err)
		}
		
		writex := ""
		writex +="$env:+=\","
		writex +=ac.PlaneType
		writex +="\" #"
		writex +=ac.Desc
		writex +="\r\n"
		
		if _, err = f.WriteString(writex); err != nil {
			panic(err)
		}
		
		f.Close()
	}
}

func PrintFlagged(allFilteredAircraft []Aircraft, config configuration.Config, filters string, marks string)  {
	for _, ac := range allFilteredAircraft {
		if ac.DbFlags > 0 && ac.DbFlags < 4  {
			PrintAC(ac,config,filters, marks)
		}
	}
}

func PrintNoType(allFilteredAircraft []Aircraft, config configuration.Config, filters string, marks string)  {
	for _, ac := range allFilteredAircraft {
		if ac.PlaneType=="" {
			//fmt.Printf("No Type: %s\r\n",ac)
			PrintAC(ac,config,filters, marks)
		}
	}
}

func PrintStrange(allFilteredAircraft []Aircraft, config configuration.Config, filters string, marks string)  {
	for _, ac := range allFilteredAircraft {
		if (ac.PlaneType=="") && (ac.OwnOp==""){// && (ac.AltGeom != 0) {
		//if (ac.PlaneType=="") {
			//Ignore Ground Vechiles
			if !( ac.Category == "C1" || ac.Category == "C2")  &&
				(ac.GS > 110){
				//fmt.Printf("No Type: %s\r\n",ac)
				PrintAC(ac,config,filters, marks)
			}
		}
	}
}


func PrintACs(allFilteredAircraft []Aircraft, config configuration.Config, filters string, marks string)  {
	for _, ac := range allFilteredAircraft {
		PrintAC(ac,config,filters, marks)
	}
}
func PrintAC(ac Aircraft, config configuration.Config, filters string, marks string)  {
		altitude := 0
		
		altitude = ac.AltGeom
		
		
		//TrackerURL := fmt.Sprintf("https://globe.adsbexchange.com/?icao=%v&SiteLat=%f&SiteLon=%f&zoom=11&enableLabels&extendedLabels=1&noIsolation",
			//ac.ICAO, config.Location.Lat, config.Location.Lon)
		TrackerURL := fmt.Sprintf("https://globe.adsbexchange.com/?icao=%v&SiteLat=%f&SiteLon=%f&zoom=11",
			ac.ICAO, config.Location.Lat, config.Location.Lon)
		
		imageURL := ""//image.Link
		if config.PritnImage > 0{
			image := planespotter.GetImageFromAPI(ac.ICAO, ac.Registration)
			imageURL = image.Link
		} 
		
		
		Catagory := "Unknown"
		if ac.Category == "A1" {
			Catagory = "Light (< 15500 lbs)"
		} else if  ac.Category == "A2" {
			Catagory = "Small (15500 to 75000 lbs)"
		} else if  ac.Category == "A3" {
			Catagory = "Large (75000 to 300000 lbs)"
		} else if  ac.Category == "A4" {
			Catagory = "High vortex large (aircraft such as B-757)"
		} else if  ac.Category == "A5" {
			Catagory = "Heavy (> 300000 lbs) "
		} else if  ac.Category == "A6" {
			Catagory = "High performance (> 5g acceleration and 400 kts)"
		} else if  ac.Category == "A7" {
			Catagory = "Rotorcraft"
		}
		Catagory = ac.Category + " " + Catagory
		
		width, _ := consolesize.GetConsoleSize()
		

		line1 := fmt.Sprintf(" - %d - %10s - %s", ac.DbFlags, Catagory,marks)
		for len(line1) < (width - 24){
			line1 += fmt.Sprintf("%s",marks)
		}
		
		line2 := ""
		for len(line2) < (width - 3){
			line2 += fmt.Sprintf("%s",marks)
		}
		
		log.Printf("%s", line1)
		fmt.Printf("%s %s : %6s, %6s, %6s, %4s, %15s, %s\r\n",marks, filters, ac.Callsign, ac.Registration, ac.PlaneType , ac.Year, ac.Desc, ac.OwnOp )
		fmt.Printf("%s Alt: %05dft, Dist: %03.2fkm, GS: %03.0fkts, RwyBearing: %03.0fdeg, Track: %03.0fdeg\r\n", marks, altitude, ac.Dst, ac.GS, ac.Dir, ac.Track)
		fmt.Printf("%s\r\n", TrackerURL)
		if(imageURL != "" ){
			fmt.Printf("%s\r\n", imageURL)
		}
		fmt.Printf("%s\r\n",line2)
		
	
}


// CalculateDistance returns the rounded distance between two coordinates in kilometers
func CalculateDistance(source geodist.Coord, destination geodist.Coord) int {
	_, kilometers := geodist.HaversineDistance(source, destination)
	return int(kilometers)
}

func CalculateDistanceF(source geodist.Coord, destination geodist.Coord) float64 {
	_, kilometers := geodist.HaversineDistance(source, destination)
	return (kilometers)
}

// convertKilometersToNauticalMiles converts kilometers into miles. The miles are rounded.
func convertKilometersToNauticalMiles(kilometers float64) int {
	return int(kilometers / 1.852)
}

// getMilitaryAircraftInRange gets all the military aircraft on the map, loops over each aircraft and returns
// only the aircraft that are within the specified maxRangeKilometers.
func getMilitaryAircraftInRange(location geodist.Coord, maxRangeKilometers int) (aircraft []Aircraft, err error) {
	var flightData FlightData
	endpoint, err := url.JoinPath(baseURL, "mil")
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	err = json.Unmarshal(body, &flightData)
	if err != nil {
		return nil, err
	}

	for _, ac := range flightData.AC {
		distance := CalculateDistance(location, geodist.Coord{Lat: ac.Lat, Lon: ac.Lon})
		if distance <= maxRangeKilometers {
			aircraft = append(aircraft, ac)
		}
	}
	return aircraft, nil
}


// getAllAircraftInRange returns all aircraft within maxRange kilometers of the location.
func getAllAircraftInRange(urlx string, location geodist.Coord, maxRangeKilometers int) (aircraft []Aircraft, err error) {
	var flightData FlightData
	miles := convertKilometersToNauticalMiles(float64(maxRangeKilometers))
	endpoint, err := url.JoinPath(urlx, "point",
		strconv.FormatFloat(location.Lat, 'f', -1, 64),
		strconv.FormatFloat(location.Lon, 'f', -1, 64),
		strconv.Itoa(miles))
	if err != nil {
		fmt.Printf("Err1 %s\r\n",endpoint)
		return nil, err
	}
	
	//fmt.Printf("%s\r\n\r\n",endpoint)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		fmt.Printf("Err2 %s\r\n",req)
		return nil, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Err3 %s\r\n",res)
		return nil, err
	}

	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	err = json.Unmarshal(body, &flightData)
	if err != nil {
		fmt.Printf("Err4 %s\r\n",body)
		return nil, err
	}

	return flightData.AC, nil
}

// newlySpotted returns true if the aircraft has not been spotted during the last interval.
func newlySpotted(aircraft Aircraft, spottedAircraft []Aircraft) bool {
	return !containsAircraft(aircraft, spottedAircraft)
}

// containsAircraft checks if the aircraft exists in the list of aircraft.
func containsAircraft(aircraft Aircraft, aircraftList []Aircraft) bool {
	for _, ac := range aircraftList {
		if ac.ICAO == aircraft.ICAO {
			if ac.PlaneType == aircraft.PlaneType {
				return true
			} else {
				log.Printf("Plane Type Changed: %s, %s > %s", ac.Callsign, aircraft.PlaneType , ac.PlaneType)
			}
		}
	}
	return false
}

// updateSpottedAircraft removed the previously spotted aircraft that are no longer in range.
func updateSpottedAircraft(alreadySpottedAircraft, filteredAircraft []Aircraft) (aircraft []Aircraft) {
	for _, ac := range alreadySpottedAircraft {
		if containsAircraft(ac, filteredAircraft) {
			aircraft = append(aircraft, ac)
		}
	}

	return aircraft
}

// validateAircraft returns a list of aircraft that have not yet been spotted and
// a list of aircraft that are already spotted, aircraft that were previously spotted but haven't been spotted
// in the last attempt are removed from the already spotted list.
// In practice this means that if an aircraft leaves the spotting range, it is removed from the already spotted list
// and thus the next time they appear in range, a notification will be sent for that aircraft.
func validateAircraft(allFilteredAircraft []Aircraft, alreadySpottedAircraft *[]Aircraft) (newlySpottedAircraft, updatedSpottedAircraft []Aircraft) {
	for _, ac := range allFilteredAircraft {
		if newlySpotted(ac, *alreadySpottedAircraft) {
			newlySpottedAircraft = append(newlySpottedAircraft, ac)
			*alreadySpottedAircraft = append(*alreadySpottedAircraft, ac)
		}
	}

	*alreadySpottedAircraft = updateSpottedAircraft(*alreadySpottedAircraft, allFilteredAircraft)
	return newlySpottedAircraft, *alreadySpottedAircraft
}


func landingAircaft(allFilteredAircraft []Aircraft, config configuration.Config)  {
	for _, ac := range allFilteredAircraft {
		altitude := ac.AltGeom
		BearingFromLocation := ac.Dir
		runway := float64(config.RunwayHeading)
		if  (180.0+runway-BearingFromLocation) > -float64(config.RunwayApproachAngle) && //Need modulo!
			(180.0+runway-BearingFromLocation) <  float64(config.RunwayApproachAngle) && //Need modulo!
			ac.Track > runway-float64(config.RunwayApproachAngle) && //Need modulo!
			ac.Track < runway+float64(config.RunwayApproachAngle) && //Need modulo!
			altitude < 15000 && altitude > 10 &&
			ac.GS > 50{
			
			PrintAC(ac,config, "Landing", "<<<")
		}
	}
}


func departingAircaft(allFilteredAircraft []Aircraft, config configuration.Config)  {
	for _, ac := range allFilteredAircraft {
		altitude := ac.AltGeom
		BearingFromLocation := ac.Dir
		runway := float64(config.RunwayHeading)
		if  (180.0+runway-BearingFromLocation) > -float64(config.RunwayApproachAngle) && //Need modulo!
			(180.0+runway-BearingFromLocation) <  float64(config.RunwayApproachAngle) && //Need modulo!
			ac.Track > 180.0+runway-float64(config.RunwayApproachAngle) && //Need modulo!
			ac.Track < 180.0+runway+float64(config.RunwayApproachAngle) && //Need modulo!
			altitude < 15000 && altitude > 10 &&
			ac.GS > 50 {
			
			PrintAC(ac,config, "Departing", ">>>")
		}
	}
}



func handleMetrics(aircraft []AircraftOutput) {
	for _, ac := range aircraft {
		metrics.IncrementMetrics(ac.Type, ac.Description, strconv.FormatBool(ac.Military), ac.Altitude)
	}
}

func isAircraftMilitary(aircraft Aircraft) bool {
	return aircraft.DbFlags == 1
}

func isAircraftDesired(aircraft Aircraft, aircraftType string) bool {
	if aircraftType == "MILITARY" && aircraft.DbFlags == 1 {
		return true
	}

	if aircraft.PlaneType == aircraftType || aircraftType == "ALL" {
		return true
	}

	return false
}

func isAircraftFiltered(aircraft Aircraft, aircraftType string, prnt int, info string, config configuration.Config) bool {
	if (strings.Replace(aircraft.PlaneType," ","-",-1) == aircraftType){
		if prnt >0 {
			altitude := aircraft.AltGeom
			Distance := aircraft.Dst
			log.Printf("%5s Ignoreing: %5s, %5s, %5dft, %3.2fkm",info, aircraft.Callsign, aircraftType, altitude, Distance)
		}
		return true
	}

	return false
}

// filterAircraftByTypes returns a list of Aircraft that match the aircraftTypes.
func filterAircraftByTypes(aircraft []Aircraft, types []string, filters []string, prnt int, info string, config configuration.Config) []Aircraft {
	var filteredAircraft []Aircraft

	for _, ac := range aircraft {
		for _, aircraftType := range types {
			if isAircraftDesired(ac, aircraftType) {
				var filtered = false
				for _, aircraftType := range filters {
					if isAircraftFiltered(ac, aircraftType, prnt, info,config) {
						filtered = true
						break
					}
				}
				if !filtered{
					filteredAircraft = append(filteredAircraft, ac)
				}
			}
		}
	}

	return filteredAircraft
}

// filterAircraftByAltitude returns a list of Aircraft that are below the maxAltitudeFeet.
func filterAircraftByAltitude(aircraft []Aircraft, minAltitudeFeet int,maxAltitudeFeet int, prnt int) []Aircraft {
	var filteredAircraft []Aircraft

	for _, ac := range aircraft {
		if ac.AltBaro != nil {
			//log.Printf("Alt: %s", ac.AltBaro)
			altitude := ac.AltGeom
			if altitude <= maxAltitudeFeet &&  altitude >= minAltitudeFeet{
				filteredAircraft = append(filteredAircraft, ac)
			} //else {
				//if prnt > 0 {
					//log.Printf("Alt Filter: %d %d", altitude, maxAltitudeFeet)
				//}
			//}
			
		}
	}

	return filteredAircraft
}


// filterAircraftByAltitude returns a list of Aircraft that are below the maxAltitudeFeet.
func filterAircraftDistance(aircraft []Aircraft, minAltitudeFeet float64,maxAltitudeFeet float64, prnt int, config configuration.Config) []Aircraft {
	var filteredAircraft []Aircraft

	for _, ac := range aircraft {
		aircraftLocation := geodist.Coord{Lat: ac.Lat, Lon: ac.Lon}
		Distance := CalculateDistanceF(config.Location, aircraftLocation)
		if Distance <= maxAltitudeFeet {
			filteredAircraft = append(filteredAircraft, ac)
		}
	}

	return filteredAircraft
}


// ConvertKnotsToKilometersPerHour well converts knots to kilometers per hour...
func ConvertKnotsToKilometersPerHour(knots int) int {
	return int(float64(knots) * 1.852)
}

// ConvertFeetToMeters converts feet to meters, * pikachu face *
func ConvertFeetToMeters(feet float64) int {
	return int(feet * 0.3048)
}

// getCloudCoverage gets the coverage percentage of the clouds at a given altitude block
// Altitude blocks are one of the following
// low    -> 0m up to 3000m
// medium -> 3000m up to 8000m
// high   -> above 8000m
func getCloudCoverage(weather weather.Data, altitudeInFeet float64) (cloudCoveragePercentage int) {

	altitudeInMeters := ConvertFeetToMeters(altitudeInFeet)
	hourUTC := (time.Now().Hour())

	switch {
	case altitudeInMeters < 3000:
		return weather.Hourly.CloudcoverLow[hourUTC]
	case altitudeInMeters >= 3000 && altitudeInMeters < 8000:
		return getHighestValue(weather.Hourly.CloudcoverLow[hourUTC], weather.Hourly.CloudcoverMid[hourUTC])
	default:
		return getHighestValue(weather.Hourly.CloudcoverLow[hourUTC],
			weather.Hourly.CloudcoverMid[hourUTC],
			weather.Hourly.CloudcoverHigh[hourUTC])
	}
}

func getHighestValue(numbers ...int) (highest int) {
	highest = 0
	for _, v := range numbers {
		if v > highest {
			highest = v
		}
	}
	return highest
}

func validateFields(aircraft Aircraft) Aircraft {
	if aircraft.Callsign == "" || strings.HasPrefix(aircraft.Callsign, " ") {
		aircraft.Callsign = "UNKNOWN"
	}

	if aircraft.AltBaro == "groundft" || aircraft.AltBaro == "ground" || aircraft.AltBaro == nil {
		aircraft.AltBaro = float64(0)
	}

	altitudeBarometricFloat := aircraft.AltBaro.(float64)
	if altitudeBarometricFloat < 0 {
		altitudeBarometricFloat = 0
		aircraft.AltBaro = altitudeBarometricFloat
	}

	return aircraft
}

// CreateAircraftOutput returns a list of AircraftOutput objects that will be used to print metadata.
func CreateAircraftOutput(aircraft []Aircraft, config configuration.Config) (acOutputs []AircraftOutput, err error) {
	var acOutput AircraftOutput
	cloudForecastSucceeded := true

	weather, err := weather.GetCloudForecast(config.Location)
	if err != nil {
		log.Printf("Error getting cloud forecast: %v\n", err)
		cloudForecastSucceeded = false
	}

	for _, ac := range aircraft {
		ac = validateFields(ac)
		aircraftLocation := geodist.Coord{Lat: ac.Lat, Lon: ac.Lon}
		image := planespotter.GetImageFromAPI(ac.ICAO, ac.Registration)

		acOutput.Altitude = ac.AltBaro.(float64)
		acOutput.Callsign = ac.Callsign
		acOutput.Description = ac.Desc
		acOutput.Distance = CalculateDistance(config.Location, aircraftLocation)
		acOutput.Speed = int(ac.GS)
		acOutput.Registration = ac.Registration
		acOutput.Type = ac.PlaneType
		acOutput.ICAO = ac.ICAO
		acOutput.Heading = ac.Track
		acOutput.TrackerURL = fmt.Sprintf("https://globe.adsbexchange.com/?icao=%v&SiteLat=%f&SiteLon=%f&zoom=11&enableLabels&extendedLabels=1&noIsolation",
			ac.ICAO, config.Location.Lat, config.Location.Lon)
		if cloudForecastSucceeded {
			acOutput.CloudCoverage = getCloudCoverage(*weather, acOutput.Altitude)
		}
		acOutput.BearingFromLocation = CalculateBearing(config.Location, aircraftLocation)
		acOutput.BearingFromAircraft = CalculateBearing(aircraftLocation, config.Location)
		acOutput.ImageThumbnailURL = image.ThumbnailLarge.Src
		acOutput.ImageURL = image.Link
		acOutput.Military = isAircraftMilitary(ac)
		acOutputs = append(acOutputs, acOutput)
	}
	return acOutputs, nil
}

// SortByDistance sorts a slice of aircraft to show the closest aircraft first
func SortByDistance(aircraft []AircraftOutput) []AircraftOutput {
	sort.Slice(aircraft, func(i, j int) bool {
		return aircraft[i].Distance < aircraft[j].Distance
	})

	return aircraft
}

// CalculateBearing returns the bearing from the source to the target
func CalculateBearing(source geodist.Coord, target geodist.Coord) float64 {
	y := math.Sin(toRadians(target.Lon-source.Lon)) * math.Cos(toRadians(target.Lat))
	x := math.Cos(toRadians(source.Lat))*math.Sin(toRadians(target.Lat)) - math.Sin(toRadians(source.Lat))*math.Cos(toRadians(target.Lat))*math.Cos(toRadians(target.Lon-source.Lon))

	bearing := math.Atan2(y, x)
	bearing = (toDegrees(bearing) + 360)

	if bearing >= 360 {
		bearing -= 360
	}

	return bearing
}

func toRadians(degrees float64) float64 {
	return degrees * (math.Pi / 180)
}

func toDegrees(rad float64) float64 {
	return rad * (180 / math.Pi)
}
