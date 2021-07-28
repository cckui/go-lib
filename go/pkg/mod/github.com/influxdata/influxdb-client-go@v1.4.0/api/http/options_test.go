// Copyright 2020 InfluxData, Inc. All rights reserved.
// Use of this source code is governed by MIT
// license that can be found in the LICENSE file.

package http_test

import (
	"crypto/tls"
	"github.com/influxdata/influxdb-client-go/api/http"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDefaultOptions(t *testing.T) {
	opts := http.DefaultOptions()
	assert.Equal(t, (*tls.Config)(nil), opts.TLSConfig())
	assert.Equal(t, uint(20), opts.HTTPRequestTimeout())
}

func TestOptionsSetting(t *testing.T) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}
	opts := http.DefaultOptions().
		SetTLSConfig(tlsConfig).
		SetHTTPRequestTimeout(50)
	assert.Equal(t, tlsConfig, opts.TLSConfig())
	assert.Equal(t, uint(50), opts.HTTPRequestTimeout())
}
