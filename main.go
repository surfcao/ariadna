package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/gen1us2k/ariadna/importer"
	"github.com/qedus/osmpbf"
	"gopkg.in/olivere/elastic.v3"
	"io/ioutil"
	"os"
	"runtime"
)

var (
	CitiesAndTowns, Roads []importer.JsonWay
	Version               string = "dev"
	configPath            string
	indexSettingsPath     string
)

func getDecoder(file *os.File) *osmpbf.Decoder {
	decoder := osmpbf.NewDecoder(file)
	err := decoder.Start(runtime.GOMAXPROCS(-1))
	if err != nil {
		importer.Logger.Fatal(err.Error())
	}
	return decoder
}

func main() {
	app := cli.NewApp()
	app.Name = "Ariadna"
	app.Usage = "OSM Geocoder"
	app.Version = Version

	app.Commands = []cli.Command{
		{
			Name:      "import",
			Aliases:   []string{"i"},
			Usage:     "Import OSM file to ElasticSearch",
			Action:    actionImport,
			ArgsUsage: "<filename>",
		},
		{
			Name:    "update",
			Aliases: []string{"u"},
			Usage:   "Download OSM file and update index",
			Action:  actionUpdate,
		},
		{
			Name:    "http",
			Aliases: []string{"h"},
			Usage:   "Run http server",
			Action:  actionHttp,
		},
	}
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "config",
			Usage:       "Config file path",
			Destination: &configPath,
		},
		cli.StringFlag{
			Name:        "index_settings",
			Usage:       "ElasticSearch Index settings",
			Destination: &indexSettingsPath,
		},
	}

	app.Before = func(context *cli.Context) error {
		if configPath == "" {
			configPath = "config.json"
		}
		if indexSettingsPath == "" {
			indexSettingsPath = "index.json"
		}
		return nil
	}

	if err := app.Run(os.Args); err != nil {
		importer.Logger.Fatal("error on run app, %v", err)
	}


}
func actionImport(ctx *cli.Context) {
	fmt.Println(configPath)
	importer.ReadConfig(configPath)
	indexSettings, err := ioutil.ReadFile(indexSettingsPath)
	if err != nil {
		importer.Logger.Fatal(err.Error())
	}
	db := importer.OpenLevelDB("db")
	defer db.Close()

	file := importer.OpenFile(importer.C.FileName)
	fmt.Println(importer.C.IndexName)
	defer file.Close()
	decoder := getDecoder(file)

	client, err := elastic.NewClient()
	if err != nil {
		// Handle error
	}
	_, err = client.CreateIndex(importer.C.IndexName).BodyString(string(indexSettings)).Do()
	if err != nil {
		// Handle error
		importer.Logger.Error(err.Error())
	}

	importer.Logger.Info("Searching cities, villages, towns and districts")
	tags := importer.BuildTags("place~city,place~village,place~suburb,place~town,place~neighbourhood")
	CitiesAndTowns, _ = importer.Run(decoder, db, tags)

	importer.Logger.Info("Cities, villages, towns and districts found")

	file = importer.OpenFile(importer.C.FileName)
	defer file.Close()
	decoder = getDecoder(file)

	importer.Logger.Info("Searching addresses")
	tags = importer.BuildTags("addr:street+addr:housenumber,amenity,shop,addr:housenumber")
	AddressWays, AddressNodes := importer.Run(decoder, db, tags)
	importer.Logger.Info("Addresses found")
	importer.JsonWaysToES(AddressWays, CitiesAndTowns, client)
	importer.JsonNodesToEs(AddressNodes, CitiesAndTowns, client)
	file = importer.OpenFile(importer.C.FileName)
	defer file.Close()
	decoder = getDecoder(file)

	tags = importer.BuildTags("highway")
	Roads, _ = importer.Run(decoder, db, tags)
	importer.RoadsToPg(Roads)
	importer.Logger.Info("Searching all roads intersecitons")
	Intersections := importer.GetRoadIntersectionsFromPG()
	importer.JsonNodesToEs(Intersections, CitiesAndTowns, client)
}

func actionUpdate(ctx *cli.Context) {
	fmt.Println("Here")
}

func actionHttp(ctx *cli.Context) {
	fmt.Println("Start http server")
}
