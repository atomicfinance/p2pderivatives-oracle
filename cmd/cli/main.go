package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"p2pderivatives-oracle/internal/database/entity"

	conf "github.com/cryptogarageinc/server-common-go/pkg/configuration"
	"github.com/cryptogarageinc/server-common-go/pkg/database/orm"
	"github.com/cryptogarageinc/server-common-go/pkg/log"
	stdlog "log"
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

var (
	configPath = flag.String("config", "", "Path to the configuration file to use.")
	appName    = flag.String("appname", "", "The name of the application. Will be use as a prefix for environment variables.")
	envname    = flag.String("e", "", "environment (ex., \"development\"). Should match with the name of the configuration file.")
	migrate    = flag.Bool("migrate", false, "If set performs a db migration before starting.")
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
	createCommand := flag.NewFlagSet("create", flag.ExitOnError)

	// Subcommands
	countCommand := flag.NewFlagSet("count", flag.ExitOnError)
	listCommand := flag.NewFlagSet("list", flag.ExitOnError)

	// Count subcommand flag pointers
	// Adding a new choice for --metric of 'substring' and a new --substring flag
	countTextPtr := countCommand.String("text", "", "Text to parse. (Required)")
	countMetricPtr := countCommand.String("metric", "chars", "Metric {chars|words|lines|substring}. (Required)")
	countSubstringPtr := countCommand.String("substring", "", "The substring to be counted. Required for --metric=substring")
	countUniquePtr := countCommand.Bool("unique", false, "Measure unique values of a metric.")

	// Use flag.Var to create a flag of our new flagType
	// Default value is the current value at countStringListPtr (currently a nil value)
	var countStringList stringList
	countCommand.Var(&countStringList, "substringList", "A comma seperated list of substrings to be counted.")

	// List subcommand flag pointers
	listTextPtr := listCommand.String("text", "", "Text to parse. (Required)")
	listMetricPtr := listCommand.String("metric", "chars", "Metric <chars|words|lines>. (Required)")
	listUniquePtr := listCommand.Bool("unique", false, "Measure unique values of a metric.")

	createAssetIDPtr := createCommand.String("asset", "", "Text to parse. (Required)")
	createPublishDatePtr := createCommand.String("publishdate", "", "Text to parse. (Required)")
	createEventTypePtr := createCommand.String("eventtype", "", "Text to parse. (Required)")

	// Verify that a subcommand has been provided
	// os.Arg[0] is the main command
	// os.Arg[1] will be the subcommand
	if len(os.Args) < 2 {
		fmt.Println("list or count subcommand is required")
		os.Exit(1)
	}

	// Switch on the subcommand
	// Parse the flags for appropriate FlagSet
	// FlagSet.Parse() requires a set of arguments to parse as input
	// os.Args[2:] will be all arguments starting after the subcommand at os.Args[1]
	switch os.Args[1] {
	case "list":
		listCommand.Parse(os.Args[2:])
	case "count":
		countCommand.Parse(os.Args[2:])
	case "create":
		createCommand.Parse(os.Args[2:])
	default:
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Check which subcommand was Parsed using the FlagSet.Parsed() function. Handle each case accordingly.
	// FlagSet.Parse() will evaluate to false if no flags were parsed (i.e. the user did not provide any flags)
	if listCommand.Parsed() {
		// Required Flags
		if *listTextPtr == "" {
			listCommand.PrintDefaults()
			os.Exit(1)
		}
		//Choice flag
		metricChoices := map[string]bool{"chars": true, "words": true, "lines": true}
		if _, validChoice := metricChoices[*listMetricPtr]; !validChoice {
			listCommand.PrintDefaults()
			os.Exit(1)
		}
		// Print
		fmt.Printf("textPtr: %s, metricPtr: %s, uniquePtr: %t\n",
			*listTextPtr,
			*listMetricPtr,
			*listUniquePtr,
		)
	}

	if countCommand.Parsed() {
		// Required Flags
		if *countTextPtr == "" {
			countCommand.PrintDefaults()
			os.Exit(1)
		}
		// If the metric flag is substring, the substring or substringList flag is required
		if *countMetricPtr == "substring" && *countSubstringPtr == "" && (&countStringList).String() == "[]" {
			countCommand.PrintDefaults()
			os.Exit(1)
		}
		//If the metric flag is not substring, the substring flag must not be used
		if *countMetricPtr != "substring" && (*countSubstringPtr != "" || (&countStringList).String() != "[]") {
			fmt.Println("--substring and --substringList may only be used with --metric=substring.")
			countCommand.PrintDefaults()
			os.Exit(1)
		}
		//Choice flag
		metricChoices := map[string]bool{"chars": true, "words": true, "lines": true, "substring": true}
		if _, validChoice := metricChoices[*listMetricPtr]; !validChoice {
			countCommand.PrintDefaults()
			os.Exit(1)
		}
		//Print
		fmt.Printf("textPtr: %s, metricPtr: %s, substringPtr: %v, substringListPtr: %v, uniquePtr: %t\n",
			*countTextPtr,
			*countMetricPtr,
			*countSubstringPtr,
			(&countStringList).String(),
			*countUniquePtr,
		)
	}

	if createCommand.Parsed() {
		// Required Flags
		if *createAssetIDPtr == "" {
			createCommand.PrintDefaults()
			os.Exit(1)
		}
		if *createPublishDatePtr == "" {
			createCommand.PrintDefaults()
			os.Exit(1)
		}
		if *createEventTypePtr == "" {
			createCommand.PrintDefaults()
			os.Exit(1)
		}

		// first find asset by asset ID provided to CLI
		asset, err := entity.FindAsset(db, *createAssetIDPtr)
		if err != nil {
			countCommand.PrintDefaults()
			os.Exit(1)
		}
		fmt.Println(asset.AssetID)
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
