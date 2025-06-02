package insightops_logrus

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)           // This will affect your stdout level, but not the level for the insightops hook. You specify that priority on creation
	logrus.SetFormatter(&logrus.JSONFormatter{}) // You can use any formatter; the hook will always format as JSON without interfering with your other hooks

	hook, err := New(
		"00000000-0000-0000-0000-000000000000", // fetching token from env vars here. You can make a token in your insightops account and are expected to have 1 token for each application
		"eu",
		&Opts{
			Priority: logrus.InfoLevel, // log level is inclusive. Setting to logrus.ErrorLevel, for example, would include errors, panics, and fatals, but not info or debug.
		},
	)
	if err != nil {
		panic(err)
	}
	logrus.AddHook(hook)
}

func TestDebug(t *testing.T) {
	logrus.Debug("Test debug entry that wont appear due to log-level set")
}

func TestInfo(t *testing.T) {
	logrus.WithField("fieldb", "some random text").Info("Info level should show based on log-level set")
}

func TestError(t *testing.T) {
	logrus.WithField("some-field", "an error to log").Error("Error level should show based on log-level set")
}

func TestHandlePanic(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"bool-field": true,
				"err":        fmt.Sprintf("%+v", err),
				"number":     100,
			}).Error("Panic recovery succeeded")
		}
	}()

	logrus.WithFields(logrus.Fields{
		"service-name": "test-service",
		"number":       1,
	}).Debug("Debug with fields should log fields")

	logrus.WithFields(logrus.Fields{
		"service-name": "test-service",
		"number":       1,
	}).Info("Info with fields should log fields")

	logrus.WithFields(logrus.Fields{
		"service-name": "test-service",
		"number":       1,
		"is-bool":      true,
	}).Warn("Warn with fields should log fields")

	logrus.WithFields(logrus.Fields{
		"number": -4,
	}).Debug("Debug with fields should log fields")

	logrus.WithFields(logrus.Fields{
		"service-name": "test-service",
		"number":       1,
		"is-bool":      true,
	}).Panic("Panic level logging should log with fields")
}

func TestFakeOpInsightDataHub(t *testing.T) {
	// Create the mock TCP server
	s := createMockDataHubTcpService()

	// Create a TCP client and connect to the mock server
	conn, err := net.Dial("tcp", "localhost:514")
	if err != nil {
		t.Fatalf("Failed to connect to mock TCP server: %v", err)
	}
	defer conn.Close()

	// Create a new logrus logger and add the insightops hook
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.JSONFormatter{})

	hook, err := New(
		"00000000-0000-0000-0000-000000000000",
		"eu",
		&Opts{
			Priority: logrus.InfoLevel,
			DatahubConfig: &UnencryptedConnectionConfig{
				Type: "tcp",
				Port: 514,
				Host: "localhost",
			},
		},
	)
	if err != nil {
		panic(err)
	}
	logrus.AddHook(hook)

	ValidateMessage(t, s, "This is a debug entry that should show in logentries", logrus.DebugLevel)
	s.Stop()

	s = createMockDataHubTcpService()
	ValidateMessage(t, s, "This is a info entry that should show in logentries", logrus.InfoLevel)
	s.Stop()

	s = createMockDataHubTcpService()
	ValidateMessage(t, s, "This is a error entry that should show in logentries", logrus.ErrorLevel)
	s.Stop()
}

func ValidateMessage(t *testing.T, server *Server, messageToSend string, level logrus.Level) {
	go func() {
		for {
			select {
			case data := <-server.data:
				tmpData := string(data)
				if tmpData != "" {
					assert.Contains(t, tmpData, "00000000-0000-0000-0000-000000000000", "Message should contain token")
					assert.Contains(t, tmpData, messageToSend, "Message should match what's sent")
					break
				}
			}
		}
	}()

	switch level {
	case logrus.DebugLevel:
		logrus.Debug(messageToSend)
	case logrus.InfoLevel:
		logrus.Info(messageToSend)
	case logrus.WarnLevel:
		logrus.Warn(messageToSend)
	case logrus.ErrorLevel:
		logrus.Error(messageToSend)
	case logrus.FatalLevel:
		logrus.Fatal(messageToSend)
	case logrus.PanicLevel:
		logrus.Panic(messageToSend)
	}
}

type Server struct {
	listener net.Listener
	quit     chan interface{}
	data     chan []byte
	wg       sync.WaitGroup
}

func (s *Server) serve() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.quit:
				return
			default:
				log.Println("accept error", err)
			}
		} else {
			s.wg.Add(1)
			go func() {
				s.handleConnection(conn)
				s.wg.Done()
			}()
		}
	}
}

func createMockDataHubTcpService() *Server {
	s := &Server{
		quit: make(chan interface{}),
		data: make(chan []byte, 1),
	}

	l, err := net.Listen("tcp", "localhost:514")
	if err != nil {
		log.Fatal(err)
	}
	s.listener = l
	s.wg.Add(1)
	go s.serve()
	return s
}

func (s *Server) Stop() {
	close(s.quit)
	s.listener.Close()
	s.wg.Wait()
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 2048)
ReadLoop:
	for {
		select {
		case <-s.quit:
			return
		default:
			_ = conn.SetDeadline(time.Now().Add(200 * time.Millisecond))
			n, err := conn.Read(buf)
			if err != nil {
				if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
					continue ReadLoop
				} else if err != io.EOF {
					log.Println("read error", err)
					return
				}
			}
			if n == 0 {
				return
			}
			s.data <- buf[:n]
		}
	}
}
