package insightops_logrus

import (
	"crypto/tls"
	"fmt"
	"github.com/sirupsen/logrus"
	"net"
	"os"
	"time"
)

// InsightOpsHook used to send logs to insightOps (rapid7) formally logentries
type InsightOpsHook struct {
	encrypt   bool
	token     string
	levels    []logrus.Level
	formatter *logrus.JSONFormatter
	network   string
	port      int
	tlsConfig *tls.Config
	host      string
}

// Opts is a set of optional parameters for NewEncryptedHook
type Opts struct {
	Priority      logrus.Level                 // defaults to logrus.DebugLevel (include all), logging level is inclusive
	TlsConfig     *tls.Config                  // defaults to use system's cert store; only needed if you need to use your own root certs
	DatahubConfig *UnencryptedConnectionConfig // useful if you're using an agent to proxy requests (hub)
}

type UnencryptedConnectionConfig struct {
	Type string `default:"tcp"` // defaults to tcp; valid options are tcp and udp
	Port int    `default:"514"` // defaults to 514; valid options are 80, 514, and 10000
	Host string `default:""`    // defaults to empty string; you should specify your target host if using a hub
}

const (
	hostPostfix = ".data.logs.insight.rapid7.com"
	tlsPort     = 443
)

var (
	host = "us.data.logs.insight.rapid7.com"
)

// New
// creates and returns a `Logrus` hook for InsightOps Token-based logging
// ref: https://docs.rapid7.com/insightops/token-tcp
func New(token string, region string, options *Opts) (hook *InsightOpsHook, err error) {
	if token == "" {
		err = fmt.Errorf("unable to create new hook: a Token is required")
		return nil, err
	}
	if region == "" || (region != "eu" && region != "us") {
		err = fmt.Errorf("unable to create new hook: a Region is required and must be eu or us")
		return nil, err
	}

	// Set the target host
	host = region + hostPostfix
	hook = &InsightOpsHook{
		encrypt:   true,
		token:     token,
		levels:    logrus.AllLevels,
		formatter: &logrus.JSONFormatter{},
		network:   "tcp",
		port:      tlsPort,
	}

	if options != nil {
		hook.formatter.TimestampFormat = time.RFC3339
		hook.levels = logrus.AllLevels[:options.Priority+1]

		// Datahub config
		if options.DatahubConfig != nil {
			if options.DatahubConfig.Host == "" {
				return nil, fmt.Errorf("unable to create new hook: a Datahub config must contain a Host target")
			}
			if options.DatahubConfig.Type == "" || (options.DatahubConfig.Type != "tcp" && options.DatahubConfig.Type != "udp") {
				options.DatahubConfig.Type = "tcp"
			}
			if options.DatahubConfig.Port == 0 || (options.DatahubConfig.Port != 80 && options.DatahubConfig.Port != 514 && options.DatahubConfig.Port != 10000) {
				options.DatahubConfig.Port = 514
			}

			host = options.DatahubConfig.Host
			hook.encrypt = false
			hook.network = options.DatahubConfig.Type
			hook.port = options.DatahubConfig.Port
		}

		if hook.encrypt && options.TlsConfig != nil {
			hook.tlsConfig = options.TlsConfig
		}
	}

	// Test connection
	if conn, err := hook.netConnect(); err == nil {
		err := conn.Close()
		if err != nil {
			return nil, err
		}
	}

	return
}

// Fire formats and sends JSON entry to target service
//
//goland:noinspection GoMixedReceiverTypes
func (hook *InsightOpsHook) Fire(entry *logrus.Entry) error {
	line, err := hook.format(entry)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "unable to read entry | err: %v | entry: %+v\n", err, entry)
		return err
	}

	if err = hook.write(line); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "unable to write to conn | err: %v | line: %s\n", err, line)
	}

	return nil
}

// Levels returns the log-levels supported by this hook
//
//goland:noinspection GoMixedReceiverTypes
func (hook *InsightOpsHook) Levels() []logrus.Level {
	return hook.levels
}

// netConnect establishes a new connection which caller is responsible for closing
//
//goland:noinspection GoMixedReceiverTypes
func (hook InsightOpsHook) netConnect() (net.Conn, error) {
	// Connect to InsightOps over tls/tcp
	if hook.encrypt {
		return tls.Dial(hook.network, fmt.Sprintf("%s:%d", host, hook.port), hook.tlsConfig)
	}
	// Connect to InsightOps over udp/tcp unsecured
	return net.Dial(hook.network, fmt.Sprintf("%s:%d", host, hook.port))
}

// write creates a connection and writes the given line to InsightOps with hook.token inlined
//
//goland:noinspection GoMixedReceiverTypes
func (hook *InsightOpsHook) write(line string) (err error) {
	if conn, err := hook.netConnect(); err == nil {
		defer func(conn net.Conn) {
			err := conn.Close()
			if err != nil {
				//ignore
			}
		}(conn)
		_, err = conn.Write([]byte(hook.token + line))
	}
	return
}

// format serializes entry to JSON
func (hook InsightOpsHook) format(entry *logrus.Entry) (string, error) {
	serialized, err := hook.formatter.Format(entry)
	if err != nil {
		return "", err
	}
	str := string(serialized)
	return str, nil
}
