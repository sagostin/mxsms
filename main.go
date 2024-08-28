package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
	"mxsms/sqlog"
)

var (
	appName        = "MXSMS"                 // application name
	version        = "0.7.1"                 // version
	date           = "2024-08-24"            // build date
	build          = ""                      // build number in git repository
	configFileName = "config.json"           // configuration file name
	config         *Config                   // loaded and parsed configuration
	llog           = logrus.StandardLogger() // initialize log collection
	sglogDB        *sqlog.DB                 // SMS log
	//zabbixLog      *zabbix.Log
)

const MaxErrors = 10 // maximum allowed number of connection errors

func main() {
	var debugLevel = uint(logrus.InfoLevel)
	flag.StringVar(&configFileName, "config", configFileName, "configuration `fileName`")
	flag.UintVar(&debugLevel, "level", debugLevel, "log `level` [0-5]")
	flag.Parse() // parse application launch parameters

	logrus.SetLevel(logrus.Level(debugLevel)) // debug level
	// logrus.SetFormatter(new(logrus.JSONFormatter))
	// hook, err := logrus_syslog.NewSyslogHook("", "", syslog.LOG_INFO, "")
	// if err == nil {
	//     logrus.AddHook(hook)
	// }

	// log information about the application and version
	logEntry := llog.WithField("version", version)
	if build != "" { // add build to version number
		logEntry = logEntry.WithField("build", build)
	}
	if date != "" { // add build date to version
		logEntry = logEntry.WithField("buildDate", date)
	}
	logEntry.Info(appName)

	// start an infinite loop of reading configuration and establishing connection
	for {
		// load and parse configuration file
		logEntry := llog.WithField("filename", configFileName)
		var err error
		config, err = LoadConfig(configFileName) // load configuration file
		if err != nil {
			logEntry.WithError(err).Fatal("Error loading config")
		}
		logEntry.WithField("mx", len(config.MX)).Info("Config loaded")

		/*sglogDB, err = sqlog.Connect(config.SMSGate.MYSQL)
		if err != nil {
			logEntry.WithError(err).Fatal("Error connecting to MySQL")
		}*/
		// //zabbixLog = zabbix.New(config.SMSGate.ZabbixHost)
		config.SMSGate.SMPP.Zabbix = config.SMSGate.Zabbix

		config.MXConnect()       // start asynchronous connection to MX
		config.SMSGate.Connect() // establish connection to SMPP servers
		// initialize support for system signals and wait for it to happen...
		signal := monitorSignals(os.Interrupt, os.Kill, syscall.SIGUSR1)
		config.SMSGate.Close() // stop connection to SMPP
		config.MXClose()       // stop connection to MX servers
		sglogDB.Close()        // close connection to the log
		// check if the signal is not a signal to reread the config
		if signal != syscall.SIGUSR1 {
			llog.Info("The end")
			return // end our work
		}
		llog.Info("Reload") // reread config and start all over again
	}
}

// monitorSignals starts monitoring signals and returns a value when it receives a signal.
// The parameters are a list of signals to be tracked.
func monitorSignals(signals ...os.Signal) os.Signal {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, signals...)
	return <-signalChan
}
