package server

import (
	"crypto/tls"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/abrander/agento/configuration"
	"github.com/abrander/agento/core"
	"github.com/abrander/agento/logger"
	"github.com/abrander/agento/plugins"
	"github.com/abrander/agento/plugins/agents/hostname"
	"github.com/abrander/agento/timeseries"
	"github.com/abrander/agento/userdb"
)

type (
	Server struct {
		inventory map[string]*inventory
		http      configuration.HTTPConfiguration
		https     configuration.HTTPSConfiguration
		udp       configuration.UDPConfiguration
		secret    string
		db        userdb.Database
		tsdb      timeseries.Database
		store     core.HostStore
	}
)

func NewServer(router gin.IRouter, cfg configuration.ServerConfiguration, db userdb.Database, store core.HostStore) (*Server, error) {
	s := &Server{}

	router.Any("/report", s.reportHandler)
	router.Any("/health", s.healthHandler)

	var err error
	s.http = cfg.HTTP
	s.https = cfg.HTTPS
	s.udp = cfg.UDP
	s.secret = cfg.Secret
	s.db = db
	s.tsdb, err = timeseries.NewInfluxDb(&cfg.Influxdb)
	if err != nil {
		return nil, err
	}
	s.store = store

	s.inventory = make(map[string]*inventory)

	return s, nil
}

func (s *Server) sendToInflux(stats plugins.Results, id string) error {
	points := stats.GetPoints()

	// Add hostname tag to all points
	hostname := string(*stats["hostname"].(*hostname.Hostname))
	for _, point := range points {
		point.Tags["hostname"] = hostname

		if id != "000000000000000000000000" {
			point.Tags["id"] = id
		}
	}

	return s.tsdb.WritePoints(points)
}

func (s *Server) reportHandler(c *gin.Context) {
	if c.Request.Method != "POST" {
		c.Header("Allow", "POST")
		c.String(http.StatusMethodNotAllowed, "only POST allowed")
		return
	}

	key := c.Request.Header.Get("X-Agento-Secret")

	subject, err := s.db.ResolveKey(key)
	if err != nil {
		c.String(http.StatusForbidden, "%s", err.Error())
		return
	}
	account, ok := subject.(userdb.Account)
	if !ok {
		c.String(http.StatusForbidden, "Only account keys can report metrics")
		return
	}

	var results = plugins.Results{}

	err = c.BindJSON(&results)
	if err != nil {
		c.String(http.StatusBadRequest, "%s", err.Error())
		return
	}

	if s.store != nil {
		hostname := string(*results["hostname"].(*hostname.Hostname))
		_, err = s.store.GetHostByName(account, hostname)
		if err == userdb.ErrorNoAccess {
			c.String(http.StatusForbidden, "The hostname belongs to another account")
			return
		} else if err != nil {
			host := &core.Host{
				Name:        hostname,
				TransportID: "localtransport",
			}

			err = s.store.AddHost(account, host)
			if err != nil {
				c.String(http.StatusInternalServerError, "Cannot add host")
				return
			}
		}
	}

	err = s.sendToInflux(results, subject.GetId())
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.String(http.StatusOK, "%s", "Got it")
}

func (s *Server) healthHandler(c *gin.Context) {
	if c.Request.Method != "GET" {
		c.Header("Allow", "GET")
		c.String(http.StatusMethodNotAllowed, "only GET allowed")
		return
	}

	c.String(200, "ok")
}

func (s *Server) ListenAndServe(engine *gin.Engine) {
	addr := s.http.Bind + ":" + strconv.Itoa(int(s.http.Port))

	err := http.ListenAndServe(addr, engine)
	if err != nil {
		logger.Red("server", "ListenAndServe(%s): %s", addr, err.Error())
	} else {
		logger.Yellow("server", "Listening for http at %s", addr)
	}
}

func (s *Server) ListenAndServeTLS(engine *gin.Engine) {
	// Choose strong TLS defaults
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		},
	}

	addr := s.https.Bind + ":" + strconv.Itoa(int(s.https.Port))

	server := &http.Server{
		Addr:      addr,
		Handler:   engine,
		TLSConfig: tlsConfig}

	err := server.ListenAndServeTLS(s.https.CertPath, s.https.KeyPath)
	if err != nil {
		logger.Red("server", "ListenAndServeTLS(%s): %s", addr, err.Error())
	} else {
		logger.Yellow("server", "Listening for https at %s", addr)
	}
}
