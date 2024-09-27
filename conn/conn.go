/*
** Copyright (C) 2001-2024 Zabbix SIA
** Adaptations (C) 2024 JKU
**
** This program is free software: you can redistribute it and/or modify it under the terms of
** the GNU Affero General Public License as published by the Free Software Foundation, version 3.
**
** This program is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY;
** without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
** See the GNU Affero General Public License for more details.
**
** You should have received a copy of the GNU Affero General Public License along with this program.
** If not, see <https://www.gnu.org/licenses/>.
**/

package conn

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/johannes-kuhfuss/emberplus/asn1"
	"github.com/johannes-kuhfuss/emberplus/ember"
	"github.com/johannes-kuhfuss/emberplus/s101"
	"github.com/johannes-kuhfuss/services_utils/logger"
)

// ErrConnectionSet error when trying to cache a connection handler that already exists.
var ErrConnectionSet = errors.New("connection handler already exists")

// ConnConfig is a configuration for a connection to the database.
type ConnConfig struct {
	URI string
}

// ConnCollection is a collection of connections to the database.
// Allows managing multiple connections.
type ConnCollection struct {
	mu          sync.Mutex
	conns       map[ConnConfig]*connHandler
	callTimeout time.Duration
	keepAlive   time.Duration
	done        chan bool
}

type connHandler struct {
	mu               sync.Mutex
	conn             net.Conn
	conf             ConnConfig
	lastAccessTime   time.Time
	lastAccessTimeMu sync.Mutex
	readData         chan readResponse
	parsedData       chan parsedResponse
	expectedPath     chan string
}

type parsedResponse struct {
	element ember.ElementCollection
	err     error
}

type readResponse struct {
	data []byte
	err  error
}

// Init initializes a pre-allocated connection collection.
func (c *ConnCollection) Init(keepAlive, callTimeout int) {
	c.conns = make(map[ConnConfig]*connHandler)
	c.keepAlive = time.Duration(keepAlive) * time.Second
	c.callTimeout = time.Duration(callTimeout) * time.Second
	c.done = make(chan bool)

	go c.housekeeper(10 * time.Second)
}

// HandleRequest sends a request and reads response based on the provided connection parameters.
func (c *ConnCollection) HandleRequest(req []byte, conf ConnConfig, path string) (ember.ElementCollection, error) {
	ch, err := c.get(c.callTimeout, conf)
	if err != nil {
		return nil, fmt.Errorf("failed to get conn: %w", err)

	}

	ch.mu.Lock()
	defer ch.mu.Unlock()

	// turns on response expectation in the listener

	ch.expectedPath <- path

	err = ch.conn.SetWriteDeadline(time.Now().Add(c.callTimeout))
	if err != nil {
		return nil, fmt.Errorf("failed to set write deadline for connection: %w", err)
	}

	_, err = ch.conn.Write(req)
	if err != nil {
		cerr := c.close(conf)
		if cerr != nil {
			logger.Error("write connection clean-up failed.", cerr)
		}

		return nil, fmt.Errorf("failed to write to connection: %w", err)
	}

	data := <-ch.parsedData
	if data.err != nil {
		return nil, fmt.Errorf("failed to find element, err %s, %w", data.err.Error(), ember.ErrElementNotFound)
	}

	return data.element, nil
}

// CloseAll closes all connections in the collection.
func (c *ConnCollection) CloseAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	close(c.done)

	for conf, ch := range c.conns {
		err := ch.conn.Close()
		if err != nil {
			logger.Error("failed to close connection", err)
		}

		delete(c.conns, conf)
	}
}

// NewConnConfig creates connection configuration with provided uri string.
func NewConnConfig(rawURI string) (ConnConfig, error) {
	parsed, err := url.Parse(rawURI)
	if err != nil {
		return ConnConfig{}, fmt.Errorf("failed to parse uri: %w", err)
	}

	return ConnConfig{URI: parsed.String()}, nil
}

// close closes the connection with the provided configuration.
func (c *ConnCollection) close(conf ConnConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ch, ok := c.conns[conf]
	if !ok {
		return nil
	}

	err := ch.conn.Close()
	if err != nil {
		return fmt.Errorf("failed to close connection: %w", err)
	}

	delete(c.conns, conf)

	return nil
}

func (c *ConnCollection) get(timeout time.Duration, conf ConnConfig) (*connHandler, error) {
	logger.Debug(fmt.Sprintf("looking for connection for %s", conf.URI))

	ch := c.getConn(conf)
	if ch != nil {
		logger.Debug(fmt.Sprintf("connection found for %s", conf.URI))

		ch.updateLastAccessTime()

		return ch, nil
	}

	logger.Debug(fmt.Sprintf("creating new connection for %s", conf.URI))

	ch, err := newConn(timeout, conf)
	if err != nil {
		return nil, fmt.Errorf("failed to create conn: %w", err)
	}

	err = c.setConn(conf, ch)
	if err != nil {
		defer ch.conn.Close() //nolint:errcheck

		logger.Debug(fmt.Sprintf("closed redundant connection %s, %s", conf.URI, err.Error()))

		existing := c.getConn(conf)
		if existing == nil {
			return nil, errors.New("failed to get existing connection handler")
		}

		return existing, nil
	}

	go ch.pathReader(c)

	return ch, nil
}

// housekeeper repeatedly checks for unused connections and closes them.
func (c *ConnCollection) housekeeper(interval time.Duration) {
	ticker := time.NewTicker(interval)

	logger.Debug("starting housekeeper")

	for {
		select {
		case <-c.done:
			logger.Debug("housekeeper done")

			return
		case <-ticker.C:
			logger.Debug("house keeper tick")

			c.closeUnused()
		}
	}
}

func (c *ConnCollection) closeUnused() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for conf, conn := range c.conns {
		if time.Since(conn.getLastAccessTime()) > c.keepAlive {
			err := conn.conn.Close()
			if err != nil {
				logger.Error(fmt.Sprintf("failed to close connection: %s", conf.URI), err)
			}

			delete(c.conns, conf)
			logger.Debug(fmt.Sprintf("closed unused connection: %s", conf.URI))
		}
	}
}

// getConn concurrent connections cache getter.
func (c *ConnCollection) getConn(cc ConnConfig) *connHandler {
	c.mu.Lock()
	defer c.mu.Unlock()

	ch, ok := c.conns[cc]
	if !ok {
		return nil
	}

	return ch
}

// setConn concurrent connections cache setter.
//
// Caches the connections, and returns an error if it already exists.
func (c *ConnCollection) setConn(cc ConnConfig, ch *connHandler) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	_, ok := c.conns[cc]
	if ok {
		return ErrConnectionSet
	}

	c.conns[cc] = ch

	return nil
}

//nolint:cyclop
func (ch *connHandler) read() ([]byte, error) {
	var (
		s101s          [][]byte
		incompleteS101 []byte
		out            []byte
		multi          bool
	)

	for {
		//nolint:makezero
		// length taken from Ember+ documentation
		response := make([]byte, 1290)

		n, err := ch.conn.Read(response)
		if err != nil {
			return nil, fmt.Errorf("failed to read from connection: %w", err)
		}

		if len(incompleteS101) > 0 {
			response = append(incompleteS101, response[:n]...)
		}

		s101s, incompleteS101, err = s101.GetS101s(response)
		if err != nil {
			return nil, fmt.Errorf("failed to get s101 data from read: %w", err)
		}

		if len(incompleteS101) > 0 {
			continue
		}

		glow, lastPacketType, err := s101.Decode(s101s)
		if err != nil {
			logger.Debug(fmt.Sprintf("failed to decode response: %s", err.Error()))

			continue
		}

		// Trace Message
		//logger.Debug(fmt.Sprintf("got packet with last packet type %x and data %x", lastPacketType, response))

		switch lastPacketType {
		case s101.FirstMultiPacket, s101.BodyMultiPacket:
			out = append(out, glow...)
			multi = true

			continue
		case s101.LastMultiPacket:
			out = append(out, glow...)

			return out, nil
		default:
			if multi {
				logger.Error(fmt.Sprintf("dropping message in the middle of a multi packet read %x", glow), err)

				continue
			}

			return glow, nil
		}
	}
}

// updateLastAccessTime updates the last time a connection was accessed.
func (ch *connHandler) updateLastAccessTime() {
	ch.lastAccessTimeMu.Lock()
	defer ch.lastAccessTimeMu.Unlock()

	ch.lastAccessTime = time.Now()
}

// getLastAccessTime returns the last time a connection was accessed.
func (ch *connHandler) getLastAccessTime() time.Time {
	ch.lastAccessTimeMu.Lock()
	defer ch.lastAccessTimeMu.Unlock()

	return ch.lastAccessTime
}

func (ch *connHandler) pathReader(c *ConnCollection) {
	go ch.reader(c)

	for {
		select {
		case path := <-ch.expectedPath:
			logger.Debug(fmt.Sprintf("got path for request %s", path))
			ch.parsedData <- ch.readExpected(path, c.callTimeout)
		case resp, ok := <-ch.readData:
			if !ok {
				// incase we get an error in readExpected, then we will exit this function here. As ch.reader will be
				// stopped and ch.readData chan will be closed.
				return
			}

			if resp.err != nil {
				logger.Debug(fmt.Sprintf("stopping pathReader for connection %s, err: %s", ch.conf.URI, resp.err.Error()))

				return
			}

			// Trace Message
			//logger.Debug("got not requested ember+ plus data, skipping")
		}
	}
}

func (ch *connHandler) reader(c *ConnCollection) {
	for {
		data, err := ch.read()
		ch.readData <- readResponse{data, err}

		if err != nil {
			logger.Debug(fmt.Sprintf("stopping reader for connection %s, err: %s", ch.conf.URI, err.Error()))

			cerr := c.close(ch.conf)
			if cerr != nil {
				logger.Error("reader connection clean-up failed, err: %w", cerr)
			}

			close(ch.readData)

			return
		}
	}
}

func (ch *connHandler) readExpected(path string, timeout time.Duration) parsedResponse {
	t := time.NewTimer(timeout)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			logger.Debug(fmt.Sprintf("failed to find Ember+ response in time for request with path %s", path))

			return parsedResponse{nil, errors.New("failed to find Ember+ response in time")}
		case resp := <-ch.readData:
			if resp.err != nil {
				logger.Debug(fmt.Sprintf("stopping reader for connection %s, err: %s", ch.conf.URI, resp.err.Error()))

				return parsedResponse{nil, fmt.Errorf("failed to read Ember+ response: %w", resp.err)}
			}

			el, gotPath, err := ch.getCollection(resp.data)
			if err != nil {
				logger.Debug(fmt.Sprintf("failed to read glow response: %s", err.Error()))

				continue
			}

			if !ch.expectedData(path, gotPath) {
				continue
			}

			// Trace Message
			//logger.Debug(fmt.Sprintf("found expected response with path %s", path))

			return parsedResponse{el, nil}
		}
	}
}

func (ch *connHandler) getCollection(glow []byte) (ember.ElementCollection, []string, error) {
	el := ember.NewElementConnection()

	err := el.Populate(asn1.NewDecoder(glow))
	if err != nil {
		return ember.ElementCollection{}, nil, fmt.Errorf("failed to populate glow response: %w", err)
	}

	if len(el) == 0 {
		return ember.ElementCollection{}, nil, errors.New("empty collection")
	}

	var gotPath []string

	// Trace Message
	//logger.Debug(fmt.Sprintf("got collection, %+v", el))

	for k := range el {
		// we care only about the path from the one element as it's a control value and every other element
		// should have the same path prefix
		gotPath = strings.Split(k.Path, ".")
		// Trace Message
		//logger.Debug(fmt.Sprintf("path from first element %s", gotPath))

		break
	}

	return el, gotPath, nil
}

func (ch *connHandler) expectedData(expectedPath string, incomingPath []string) bool {
	splitExpectedPath, expectedLength := parseExpectedLength(expectedPath)
	// gotPath has to be one path element longer or the same length in case data has children and not values
	if len(incomingPath) != expectedLength && len(incomingPath) != expectedLength+1 {
		// Trace Message
		logger.Debug(fmt.Sprintf("path %s length %d does not match the expected path %s length %d", incomingPath, len(incomingPath), splitExpectedPath, expectedLength))

		return false
	}

	for i, v := range splitExpectedPath {
		if incomingPath[i] != v {
			// Trace Message
			logger.Debug(fmt.Sprintf("path %s does not match the expected %s", incomingPath, splitExpectedPath))

			return false
		}
	}

	return true
}

func parseExpectedLength(path string) ([]string, int) {
	if path == "" {
		return nil, 1
	}

	splitExpectedPath := strings.Split(path, ".")

	return splitExpectedPath, len(splitExpectedPath)
}

func newConn(timeout time.Duration, conf ConnConfig) (*connHandler, error) {
	u, err := url.Parse(conf.URI)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URI: %w", err)
	}

	d := &net.Dialer{Timeout: timeout, KeepAlive: 4 * time.Second}

	conn, err := d.Dial("tcp", u.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection: %w", err)
	}

	return &connHandler{
		conn:           conn,
		lastAccessTime: time.Now(),
		conf:           conf,
		readData:       make(chan readResponse),
		parsedData:     make(chan parsedResponse),
		expectedPath:   make(chan string),
	}, nil
}
