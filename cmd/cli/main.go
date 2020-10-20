package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"p2pderivatives-oracle/internal/database/entity"

	conf "github.com/cryptogarageinc/server-common-go/pkg/configuration"
	"github.com/cryptogarageinc/server-common-go/pkg/database/orm"
	"github.com/cryptogarageinc/server-common-go/pkg/log"
	"github.com/jinzhu/gorm"
	stdlog "log"
	"p2pderivatives-oracle/internal/dlccrypto"
	"p2pderivatives-oracle/internal/oracle"
)

// "p2pderivatives-oracle/internal/api/asset_controller"

// Create a new type for a list of Strings
type stringList []string

// Implement the flag.Value interface
func (s *stringList) String() string {
	return fmt.Sprintf("%v", *s)
}

func (s *stringList) Set(value string) error {
	*s = strings.Split(value, ",")
	return nil
}

const (
	// TimeFormatISO8601 time format of the api using ISO8601
	TimeFormatISO8601 = "2006-01-02T15:04:05Z"
)

var (
	configPath  = flag.String("config", "", "Path to the configuration file to use.")
	appName     = flag.String("appname", "", "The name of the application. Will be use as a prefix for environment variables.")
	envname     = flag.String("e", "", "environment (ex., \"development\"). Should match with the name of the configuration file.")
	migrate     = flag.Bool("migrate", false, "If set performs a db migration before starting.")
	action      = flag.String("action", "", "Action")
	asset       = flag.String("asset", "", "Asset")
	publishdate = flag.String("publishdate", "", "Publish Date")
	eventtype   = flag.String("eventtype", "", "Event Type")
	outcome     = flag.String("outcome", "", "Outcome")
)

// Config contains the configuration parameters for the server.
type Config struct {
	Address  string `configkey:"server.address" validate:"required"`
	TLS      bool   `configkey:"server.tls"`
	CertFile string `configkey:"server.certfile" validate:"required_with=TLS"`
	KeyFile  string `configkey:"server.keyfile" validate:"required_with=TLS"`
}

func init() {
	flag.Parse()

	if *configPath == "" {
		stdlog.Fatal("No configuration path specified")
	}

	if *appName == "" {
		stdlog.Fatal("No configuration name specified")
	}

	if *envname != "" {
		os.Setenv("P2PD_ENV", *envname)
	}
}

func main() {

	config := conf.NewConfiguration(*appName, *envname, []string{*configPath})
	err := config.Initialize()

	if err != nil {
		stdlog.Fatalf("Could not read configuration %v.", err)
	}

	logInstance := newInitializedLog(config)
	// log := logInstance.Logger

	// Setup orm service
	ormInstance := newInitializedOrm(config, logInstance)

	db := ormInstance.GetDB()

	// Subcommands
	countCommand := flag.NewFlagSet("count", flag.ExitOnError)

	// Verify that a subcommand has been provided
	// os.Arg[0] is the main command
	// os.Arg[1] will be the subcommand
	if len(os.Args) < 2 {
		fmt.Println("list or count subcommand is required")
		os.Exit(1)
	}

	if *action == "create" {
		asset, err := entity.FindAsset(db, *asset)
		if err != nil {
			countCommand.PrintDefaults()
			os.Exit(1)
		}

		requestedPublishDate, err := ParseTime(*publishdate)
		if err != nil {
			countCommand.PrintDefaults()
			os.Exit(1)
		}

		fmt.Println(asset)
		fmt.Println(asset.Description)

		dlcData, err := entity.FindDLCDataPublishedAt(db, asset.AssetID, *requestedPublishDate, *eventtype)
		if err == nil {
			fmt.Println("Found a matchin DLC Data in db")
			countCommand.PrintDefaults()
			os.Exit(1)
		}
		if err != nil && !gorm.IsRecordNotFoundError(err) {
			fmt.Println("Unknown DB Error")
			countCommand.PrintDefaults()
			os.Exit(1)
		}

		cryptoInstance := dlccrypto.NewCfdgoCryptoService()

		// if record is not found, need to create the record in db
		if err != nil && gorm.IsRecordNotFoundError(err) {
			fmt.Println("Generating new DLC data Rvalue")

			signingK, err := cryptoInstance.GenerateKvalue()
			if err != nil {
				fmt.Println("Unknown Crypto Service Error: ", err)
				countCommand.PrintDefaults()
				os.Exit(1)
			}
			fmt.Println("signingK", signingK)
			rvalue, err := cryptoInstance.ComputeRvalue(signingK)
			if err != nil {
				fmt.Println("Unknown Crypto Service Error: ", err)
				countCommand.PrintDefaults()
				os.Exit(1)
			}
			dlcData, err = entity.CreateDLCData(
				db,
				asset.AssetID,
				*requestedPublishDate,
				*eventtype,
				signingK.EncodeToString(),
				rvalue.EncodeToString())
			if err != nil {
				// need to retry to be sure a concurrent didn't try to create same DLCData
				inDb, errFind := entity.FindDLCDataPublishedAt(db, asset.AssetID, *requestedPublishDate, *eventtype)
				if errFind != nil {
					fmt.Println("Unknown DB Error: ", err)
					countCommand.PrintDefaults()
					os.Exit(1)
				}
				dlcData = inDb
			}
		}

		fmt.Println("dlcData", dlcData)
	}

	if *action == "sign" {
		asset, err := entity.FindAsset(db, *asset)
		if err != nil {
			countCommand.PrintDefaults()
			os.Exit(1)
		}

		requestedPublishDate, err := ParseTime(*publishdate)
		if err != nil {
			countCommand.PrintDefaults()
			os.Exit(1)
		}

		fmt.Println(asset)
		fmt.Println(asset.Description)

		dlcData, err := entity.FindDLCDataPublishedAt(db, asset.AssetID, *requestedPublishDate, *eventtype)
		if err != nil {
			fmt.Println("Unknown find DLC Error: ", err)
			os.Exit(1)
		}
		if !dlcData.IsSigned() {
			fmt.Println("Computing Signature")

			var valueMessage string
			valueMessage = *outcome

			// Setup Oracle
			oracleConfig := &oracle.Config{}
			config.InitializeComponentConfig(oracleConfig)
			oracleInstance, err := oracle.FromConfig(oracleConfig)
			if err != nil {
				fmt.Println("Could not create a oracle instance, Error: ", err)
				os.Exit(1)
			}

			cryptoInstance := dlccrypto.NewCfdgoCryptoService()

			kvalue, err := dlccrypto.NewPrivateKey(dlcData.Kvalue)
			if err != nil {
				fmt.Println("Unknown Crypto Service Error: ", err)
				os.Exit(1)
			}
			sig, err := cryptoInstance.ComputeSchnorrSignature(oracleInstance.PrivateKey, kvalue, valueMessage)
			if err != nil {
				fmt.Println("Unknown Crypto Service Error: ", err)
				os.Exit(1)
			}

			dlcData, err = entity.UpdateDLCDataSignatureAndValue(
				db,
				dlcData.AssetID,
				dlcData.PublishedDate,
				dlcData.EventType,
				sig.EncodeToString(),
				valueMessage)

			if err != nil {
				fmt.Println("Unknown DB Error: ", err)
			}
		}
	}
}

func newInitializedOrm(config *conf.Configuration, log *log.Log) *orm.ORM {
	ormConfig := &orm.Config{}
	if err := config.InitializeComponentConfig(ormConfig); err != nil {
		panic(err)
	}
	ormInstance := orm.NewORM(ormConfig, log)
	err := ormInstance.Initialize()

	if err != nil {
		panic("Could not initialize database.")
	}

	return ormInstance
}

func newInitializedLog(config *conf.Configuration) *log.Log {
	logConfig := &log.Config{}
	config.InitializeComponentConfig(logConfig)
	logger := log.NewLog(logConfig)
	logger.Initialize()
	return logger
}

// ParseTime will try to parse a string using ISO8691 format and convert it to a time.Time
func ParseTime(timeParam string) (*time.Time, error) {
	timestamp, err := time.Parse(TimeFormatISO8601, timeParam)
	if err != nil {
		fmt.Println(err, "Invalid time format ! You should use ISO8601 ex: %s", TimeFormatISO8601)
		os.Exit(1)
		return nil, err
	}
	utc := timestamp.UTC()
	return &utc, nil
}
