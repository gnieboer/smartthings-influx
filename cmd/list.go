/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"log"

	"github.com/gnieboer/smartthings-influx/pkg/smartthings"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List elements from SmartThings",
	Long: `Query SmartThings for:
	   - Devices
	   - Capabilities
	   `,
	Run: func(cmd *cobra.Command, args []string) {
		convMap, err := smartthings.ParseConversionMap(viper.GetStringMap("valuemap"))
		if err != nil {
			log.Fatalf("Error initializing SmartThings client: %v", err)
		}

		smartthings.Init(viper.GetString("apitoken"), convMap)
		list, err := smartthings.Devices()

		if err != nil {
			log.Fatal(err)
		}

		for i, d := range list.Items {
			fmt.Printf("%d: %s, %s, %s\n", i, d.DeviceId, d.Name, d.Label)
			for _, comp := range d.Components {
				for _, cap := range comp.Capabilities {
					fmt.Printf("   | %s\n", cap.Id)
				}
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(listCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// listCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// listCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
