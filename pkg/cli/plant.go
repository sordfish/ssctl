package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"ssctl/pkg/kube"
	"ssctl/pkg/sunsynk"
	"ssctl/pkg/utils"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type LineFormat struct {
	Value     float64
	Name      string
	Unit      string
	PlantId   int
	Timestamp int64
}

// plantCmd represents the plant command
var plantCmd = &cobra.Command{
	Use:   "plant",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {

		debugFlagValue, _ := cmd.Parent().PersistentFlags().GetBool("debug")

		if debugFlagValue {
			os.Setenv("SS_DEBUG", "TRUE")
		}

		k8sFlagValue, _ := cmd.Parent().PersistentFlags().GetBool("k8s")
		uploadFlagValue, _ := cmd.Flags().GetBool("upload")

		pdata := Plant(k8sFlagValue)

		if uploadFlagValue {
			Upload2influxdb(pdata)
		} else {
			fmt.Println(pdata)
		}

	},
}

func init() {
	rootCmd.AddCommand(plantCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// plantCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	plantCmd.Flags().BoolP("upload", "u", false, "Enable upload to influxdb")
}

func Plant(k8s bool) string {

	var plantDataLineString, SunsynkToken, SunsynkPlantId string

	today := time.Now().Format("2006-01-02")
	dateOverride := os.Getenv("SS_DATE")

	if dateOverride != "" {
		log.Println("Date override", dateOverride)
		today = dateOverride
	}

	if !k8s {

		SunsynkPlantId = os.Getenv("SS_PLANT_ID")
		SunsynkToken = os.Getenv("SS_TOKEN")

		if SunsynkPlantId == "" {
			log.Fatal("No plant ID found in env")
		}

		if SunsynkToken == "" {
			log.Fatal("No token found in env")
		}

	} else {

		clientset, err := kube.Login()
		if err != nil {
			log.Fatal(err)
		}

		result, err := kube.GetK8sSecret(clientset, "sunsynk-token", "sunsynk")
		if err != nil {
			log.Fatal(err)
		}

		token, ok := result.Data["token"]
		if !ok {
			log.Fatal("token not found in secret data")
		}

		SunsynkToken = string(token)

		result, err = kube.GetK8sSecret(clientset, "sunsynk-user-plants", "sunsynk")
		if err != nil {
			log.Fatal(err)
		}

		plantdata, ok := result.Data["plants.json"]
		if !ok {
			log.Fatal("plants.json not found in secret data")
		}

		var UserPlantsStruct []sunsynk.SSApiUserPlant

		err = json.Unmarshal(plantdata, &UserPlantsStruct)
		if err != nil {
			log.Fatal(err)
		}

		SunsynkPlantId = fmt.Sprint(UserPlantsStruct[0].Id)

	}

	plantdata, err := sunsynk.GetPlantData(today, SunsynkPlantId, SunsynkToken)
	if err != nil {
		log.Fatal(err)
	}

	SunsynkPlantIdInt, err := strconv.Atoi(SunsynkPlantId)
	if err != nil {
		log.Fatal(err)
	}

	output, err := Plant2Line(today, SunsynkPlantIdInt, plantdata)
	if err != nil {
		log.Fatal(err)
	}

	plantDataLineString = strings.Join(output, "\n")

	return plantDataLineString

}

func Plant2Line(date string, plantID int, ssplantdata []byte) ([]string, error) {

	var plantdatastruct sunsynk.SSApiPlantDataResponse

	var plantDataLineStruct []LineFormat

	err := json.Unmarshal(ssplantdata, &plantdatastruct)
	if err != nil {
		log.Fatal(err)
	}

	for _, types := range plantdatastruct.Data.Infos {

		for _, datum := range types.Records {

			var row LineFormat
			row.Name = types.Label
			row.PlantId = plantID
			row.Unit = types.Unit

			row.Value, err = strconv.ParseFloat(datum.Value, 32)
			if err != nil {
				log.Fatal(err)
			}

			dateTimeStr := date + "T" + datum.Time + ":00Z" // combine date and time strings

			// Parse dateTimeStr into a time.Time struct
			dateTime, err := time.Parse(time.RFC3339, dateTimeStr)
			if err != nil {
				log.Fatal("Error parsing date and time:", err)
			}

			row.Timestamp = dateTime.Unix()
			plantDataLineStruct = append(plantDataLineStruct, row)
		}

	}

	var plantDataLineStringSlice []string

	for _, line := range plantDataLineStruct {
		plantDataLineStringSlice = append(plantDataLineStringSlice, "sunsynk_plant,plant="+fmt.Sprint(line.PlantId)+" "+strings.ToLower(line.Name)+"="+strconv.FormatFloat(line.Value, 'f', 2, 64)+" "+fmt.Sprint(line.Timestamp))
	}

	// sunsynk_mppt_1,plant=123456 voltage=206,current=4 1682017085

	return plantDataLineStringSlice, err

}

func Upload2influxdb(data string) {

	InfluxdbUrl := os.Getenv("INFLUXDB_URL")

	if InfluxdbUrl == "" {
		log.Fatal("InfluxDB not url set")
	}

	url := InfluxdbUrl + "/write"

	headers := map[string]string{}
	body := []byte(data)
	token := ""
	respBody, err := utils.SendHTTPRequest("POST", url, headers, body, token)
	if err != nil {
		log.Fatal(err)
	}

	log.Println(respBody)

}