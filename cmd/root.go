/*
 */
package cmd

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/senzing/go-common/g2engineconfigurationjson"
	"github.com/senzing/go-grpcing/grpcurl"
	"github.com/senzing/go-observing/observer"
	"github.com/senzing/go-rest-api-service/senzingrestservice"
	"github.com/senzing/senzing-tools/constant"
	"github.com/senzing/senzing-tools/envar"
	"github.com/senzing/senzing-tools/help"
	"github.com/senzing/senzing-tools/helper"
	"github.com/senzing/senzing-tools/option"
	"github.com/senzing/serve-http/httpserver"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

const (
	Short string = "serve-http short description"
	Use   string = "serve-http"
	Long  string = `
serve-http long description.
	`
)

// ----------------------------------------------------------------------------
// Context variables
// ----------------------------------------------------------------------------

var ContextBools = []struct {
	Dfault bool
	Envar  string
	Help   string
	Option string
}{
	{
		Dfault: false,
		Envar:  envar.EnableSwaggerUi,
		Help:   help.EnableSwaggerUi,
		Option: option.EnableSwaggerUi,
	},
	{
		Dfault: false,
		Envar:  envar.EnableAll,
		Help:   help.EnableAll,
		Option: option.EnableAll,
	},
	{
		Dfault: false,
		Envar:  envar.EnableSenzingRestApi,
		Help:   help.EnableSenzingRestApi,
		Option: option.EnableSenzingRestApi,
	},
	{
		Dfault: false,
		Envar:  envar.EnableXterm,
		Help:   help.EnableXterm,
		Option: option.EnableXterm,
	},
}

var ContextInts = []struct {
	Dfault int
	Envar  string
	Help   string
	Option string
}{
	{
		Dfault: 0,
		Envar:  envar.EngineLogLevel,
		Help:   help.EngineLogLevel,
		Option: option.EngineLogLevel,
	},
	{
		Dfault: 8261,
		Envar:  envar.HttpPort,
		Help:   help.HttpPort,
		Option: option.HttpPort,
	},
	{
		Dfault: 10,
		Envar:  envar.XtermConnectionErrorLimit,
		Help:   help.XtermConnectionErrorLimit,
		Option: option.XtermConnectionErrorLimit,
	},
	{
		Dfault: 20,
		Envar:  envar.XtermKeepalivePingTimeout,
		Help:   help.XtermKeepalivePingTimeout,
		Option: option.XtermKeepalivePingTimeout,
	},
	{
		Dfault: 512,
		Envar:  envar.XtermMaxBufferSizeBytes,
		Help:   help.XtermMaxBufferSizeBytes,
		Option: option.XtermMaxBufferSizeBytes,
	},
}

var ContextStrings = []struct {
	Dfault string
	Envar  string
	Help   string
	Option string
}{
	{
		Dfault: "",
		Envar:  envar.Configuration,
		Help:   help.Configuration,
		Option: option.Configuration,
	},
	{
		Dfault: "",
		Envar:  envar.DatabaseUrl,
		Help:   help.DatabaseUrl,
		Option: option.DatabaseUrl,
	},
	{
		Dfault: "",
		Envar:  envar.EngineConfigurationJson,
		Help:   help.EngineConfigurationJson,
		Option: option.EngineConfigurationJson,
	},
	{
		Dfault: fmt.Sprintf("serve-http-%d", time.Now().Unix()),
		Envar:  envar.EngineModuleName,
		Help:   help.EngineModuleName,
		Option: option.EngineModuleName,
	},
	{
		Dfault: "",
		Envar:  envar.GrpcUrl,
		Help:   help.GrpcUrl,
		Option: option.GrpcUrl,
	},
	{
		Dfault: "INFO",
		Envar:  envar.LogLevel,
		Help:   help.LogLevel,
		Option: option.LogLevel,
	},
	{
		Dfault: "serve-http",
		Envar:  envar.ObserverOrigin,
		Help:   help.ObserverOrigin,
		Option: option.ObserverOrigin,
	},
	{
		Dfault: "",
		Envar:  envar.ObserverUrl,
		Help:   help.ObserverUrl,
		Option: option.ObserverUrl,
	},
	{
		Dfault: "0.0.0.0",
		Envar:  envar.ServerAddress,
		Help:   help.ServerAddress,
		Option: option.ServerAddress,
	},
	{
		Dfault: "/bin/bash",
		Envar:  envar.XtermCommand,
		Help:   help.XtermCommand,
		Option: option.XtermCommand,
	},
}

var ContextStringSlices = []struct {
	Dfault []string
	Envar  string
	Help   string
	Option string
}{
	{
		Dfault: getDefaultAllowedHostnames(),
		Envar:  envar.XtermAllowedHostnames,
		Help:   help.XtermAllowedHostnames,
		Option: option.XtermAllowedHostnames,
	},
	{
		Dfault: []string{},
		Envar:  envar.XtermArguments,
		Help:   help.XtermArguments,
		Option: option.XtermArguments,
	},
}

// ----------------------------------------------------------------------------
// Private functions
// ----------------------------------------------------------------------------

// Since init() is always invoked, define command line parameters.
func init() {
	for _, contextBool := range ContextBools {
		RootCmd.Flags().Bool(contextBool.Option, contextBool.Dfault, fmt.Sprintf(contextBool.Help, contextBool.Envar))
	}
	for _, contextInt := range ContextInts {
		RootCmd.Flags().Int(contextInt.Option, contextInt.Dfault, fmt.Sprintf(contextInt.Help, contextInt.Envar))
	}
	for _, contextString := range ContextStrings {
		RootCmd.Flags().String(contextString.Option, contextString.Dfault, fmt.Sprintf(contextString.Help, contextString.Envar))
	}
	for _, contextStringSlice := range ContextStringSlices {
		RootCmd.Flags().StringSlice(contextStringSlice.Option, contextStringSlice.Dfault, fmt.Sprintf(contextStringSlice.Help, contextStringSlice.Envar))
	}
}

// If a configuration file is present, load it.
func loadConfigurationFile(cobraCommand *cobra.Command) {
	configuration := ""
	configFlag := cobraCommand.Flags().Lookup(option.Configuration)
	if configFlag != nil {
		configuration = configFlag.Value.String()
	}
	if configuration != "" { // Use configuration file specified as a command line option.
		viper.SetConfigFile(configuration)
	} else { // Search for a configuration file.

		// Determine home directory.

		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Specify configuration file name.

		viper.SetConfigName("serve-http")
		viper.SetConfigType("yaml")

		// Define search path order.

		viper.AddConfigPath(home + "/.senzing-tools")
		viper.AddConfigPath(home)
		viper.AddConfigPath("/etc/senzing-tools")
	}

	// If a config file is found, read it in.

	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Applying configuration file:", viper.ConfigFileUsed())
	}
}

// Configure Viper with user-specified options.
func loadOptions(cobraCommand *cobra.Command) {
	var err error = nil
	viper.AutomaticEnv()
	replacer := strings.NewReplacer("-", "_")
	viper.SetEnvKeyReplacer(replacer)
	viper.SetEnvPrefix(constant.SetEnvPrefix)

	// Bools

	for _, contextVar := range ContextBools {
		viper.SetDefault(contextVar.Option, contextVar.Dfault)
		err = viper.BindPFlag(contextVar.Option, cobraCommand.Flags().Lookup(contextVar.Option))
		if err != nil {
			panic(err)
		}
	}

	// Ints

	for _, contextVar := range ContextInts {
		viper.SetDefault(contextVar.Option, contextVar.Dfault)
		err = viper.BindPFlag(contextVar.Option, cobraCommand.Flags().Lookup(contextVar.Option))
		if err != nil {
			panic(err)
		}
	}

	// Strings

	for _, contextVar := range ContextStrings {
		viper.SetDefault(contextVar.Option, contextVar.Dfault)
		err = viper.BindPFlag(contextVar.Option, cobraCommand.Flags().Lookup(contextVar.Option))
		if err != nil {
			panic(err)
		}
	}

	// StringSlice

	for _, contextVar := range ContextStringSlices {
		viper.SetDefault(contextVar.Option, contextVar.Dfault)
		err = viper.BindPFlag(contextVar.Option, cobraCommand.Flags().Lookup(contextVar.Option))
		if err != nil {
			panic(err)
		}
	}
}

// --- Networking -------------------------------------------------------------

func getOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			panic(err)
		}
	}()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP
}

func getDefaultAllowedHostnames() []string {
	result := []string{"localhost"}
	outboundIpAddress := getOutboundIP().String()
	if len(outboundIpAddress) > 0 {
		result = append(result, outboundIpAddress)
	}
	return result
}

// ----------------------------------------------------------------------------
// Public functions
// ----------------------------------------------------------------------------

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the RootCmd.
func Execute() {
	err := RootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

// Used in construction of cobra.Command
func PreRun(cobraCommand *cobra.Command, args []string) {
	loadConfigurationFile(cobraCommand)
	loadOptions(cobraCommand)
	cobraCommand.SetVersionTemplate(constant.VersionTemplate)
}

// Used in construction of cobra.Command
func RunE(_ *cobra.Command, _ []string) error {
	var err error = nil
	ctx := context.TODO()

	// Build senzingEngineConfigurationJson.

	senzingEngineConfigurationJson := viper.GetString(option.EngineConfigurationJson)
	if len(senzingEngineConfigurationJson) == 0 {
		senzingEngineConfigurationJson, err = g2engineconfigurationjson.BuildSimpleSystemConfigurationJson(viper.GetString(option.DatabaseUrl))
		if err != nil {
			return err
		}
	}

	// Determine if gRPC is being used.

	grpcUrl := viper.GetString(option.GrpcUrl)
	grpcTarget := ""
	grpcDialOptions := []grpc.DialOption{}
	if len(grpcUrl) > 0 {
		grpcTarget, grpcDialOptions, err = grpcurl.Parse(ctx, grpcUrl)
		if err != nil {
			return err
		}
	}

	// Build observers.
	//  viper.GetString(option.ObserverUrl),

	observers := []observer.Observer{}

	// Create object and Serve.

	httpServer := &httpserver.HttpServerImpl{
		ApiUrlRoutePrefix:              "api",
		EnableAll:                      viper.GetBool(option.EnableAll),
		EnableSenzingRestAPI:           viper.GetBool(option.EnableSenzingRestApi),
		EnableSwaggerUI:                viper.GetBool(option.EnableSwaggerUi),
		EnableXterm:                    viper.GetBool(option.EnableXterm),
		GrpcDialOptions:                grpcDialOptions,
		GrpcTarget:                     grpcTarget,
		LogLevelName:                   viper.GetString(option.LogLevel),
		ObserverOrigin:                 viper.GetString(option.ObserverOrigin),
		Observers:                      observers,
		OpenApiSpecificationRest:       senzingrestservice.OpenApiSpecificationJson,
		ReadHeaderTimeout:              60 * time.Second,
		SenzingEngineConfigurationJson: senzingEngineConfigurationJson,
		SenzingModuleName:              viper.GetString(option.EngineModuleName),
		SenzingVerboseLogging:          viper.GetInt(option.EngineLogLevel),
		ServerAddress:                  viper.GetString(option.ServerAddress),
		ServerPort:                     viper.GetInt(option.HttpPort),
		SwaggerUrlRoutePrefix:          "swagger",
		XtermAllowedHostnames:          viper.GetStringSlice(option.XtermAllowedHostnames),
		XtermArguments:                 viper.GetStringSlice(option.XtermArguments),
		XtermCommand:                   viper.GetString(option.XtermCommand),
		XtermConnectionErrorLimit:      viper.GetInt(option.XtermConnectionErrorLimit),
		XtermKeepalivePingTimeout:      viper.GetInt(option.XtermKeepalivePingTimeout),
		XtermMaxBufferSizeBytes:        viper.GetInt(option.XtermMaxBufferSizeBytes),
		XtermUrlRoutePrefix:            "xterm",
	}
	err = httpServer.Serve(ctx)
	return err
}

// Used in construction of cobra.Command
func Version() string {
	return helper.MakeVersion(githubVersion, githubIteration)
}

// ----------------------------------------------------------------------------
// Command
// ----------------------------------------------------------------------------

// RootCmd represents the command.
var RootCmd = &cobra.Command{
	Use:     Use,
	Short:   Short,
	Long:    Long,
	PreRun:  PreRun,
	RunE:    RunE,
	Version: Version(),
}
