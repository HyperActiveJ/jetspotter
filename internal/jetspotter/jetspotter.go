package main

import (
	"jetspotter/internal/configuration"
	"jetspotter/internal/jetspotter"
	"jetspotter/internal/metrics"
	"jetspotter/internal/notification"
	"log"
	"time"
	"fmt"
)

func exitWithError(err error) {
	log.Fatalf("Something went wrong: %v\n", err)
}

func sendNotifications(aircraft []jetspotter.AircraftOutput, config configuration.Config) error {
	sortedAircraft := jetspotter.SortByDistance(aircraft)

	if len(aircraft) < 1 {
		//log.Println("No new matching aircraft have been spotted.")
		return nil
	}

	// Terminal
	notification.SendTerminalMessage(sortedAircraft, config)

	// Slack
	if config.SlackWebHookURL != "" {
		err := notification.SendSlackMessage(sortedAircraft, config)
		if err != nil {
			return err
		}
	}

	// Discord
	if config.DiscordWebHookURL != "" {
		err := notification.SendDiscordMessage(sortedAircraft, config)
		if err != nil {
			return err
		}
	}

	// Gotify
	if config.GotifyURL != "" && config.GotifyToken != "" {
		err := notification.SendGotifyMessage(sortedAircraft, config)
		if err != nil {
			return err
		}
	}

	// Ntfy
	if config.NtfyTopic != "" {
		err := notification.SendNtfyMessage(sortedAircraft, config)
		if err != nil {
			return err
		}
	}

	return nil
}

func jetspotterHandler(alreadySpottedAircraft *[]jetspotter.Aircraft, config configuration.Config) {
	//aircraft, err := jetspotter.HandleAircraft(alreadySpottedAircraft, config)
	_, err := jetspotter.HandleAircraft(alreadySpottedAircraft, config)
	if err != nil {
		log.Printf("handel")
		exitWithError(err)
	}

	//err = sendNotifications(aircraft, config)
	//if err != nil {
	//	exitWithError(err)
	//}
}

func HandleJetspotter(config configuration.Config) {
	if config.PrintStartup > 0{
		log.Printf("Requesting aircraft out %d kilometers", config.MaxRangeKilometersR)
		log.Printf("Looking For Interesting aircraft out %d kilometers from 0 to %d feet", config.MaxRangeKilometers, config.MaxAltitudeFeet)
		fmt.Printf("Including: %s \r\nExcluding: %s\r\n", config.AircraftTypes, config.AircraftTypesExcl)
		fmt.Printf("Also out %d kilometers from %d to %d feet \r\nAdditionally excluding: %s\r\n",  config.MaxRangeKilometers2, config.MaxAltitudeFeet, config.MaxAltitudeFeet2,  config.AircraftTypesExcl2)
		fmt.Printf("Also excluding from logs: %s\r\n", config.AircraftTypesKnownInteresting)
	}
	var alreadySpottedAircraft []jetspotter.Aircraft
	for {
		jetspotterHandler(&alreadySpottedAircraft, config)
		time.Sleep(time.Duration(config.FetchInterval) * time.Second/2)
		if config.RunOnce > 0{
			return
		}

	}
}

func HandleMetrics(config configuration.Config) {
	go func() {
		err := metrics.HandleMetrics(config)
		if err != nil {
			exitWithError(err)
		}
	}()
}

func main() {
	config, err := configuration.GetConfig()
	if err != nil {
		log.Printf("config")
		exitWithError(err)
	}
	//HandleMetrics(config)
	HandleJetspotter(config)
}
