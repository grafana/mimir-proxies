// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-2020 Datadog, Inc.
package ddstructs

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// APIMetricType represents an API metric type
type APIMetricType string

// Enumeration of the existing API metric types
const (
	APIGaugeType APIMetricType = "gauge"
	APIRateType  APIMetricType = "rate"
	APICountType APIMetricType = "count"
)

// Point represents a metric value at a specific time
type Point struct {
	Ts    Float64MaybeStringJSON //nolint:stylecheck
	Value Float64MaybeStringJSON
}

type Float64MaybeStringJSON float64

// UnmarshalJSON is a custom unmarshaller for Point (used for testing)
func (f *Float64MaybeStringJSON) UnmarshalJSON(buf []byte) error {
	var floatVal float64
	if err := json.Unmarshal(buf, &floatVal); err != nil {
		var strVal string
		if strUnmarshalErr := json.Unmarshal(buf, &strVal); strUnmarshalErr != nil {
			return fmt.Errorf("can't unmarshal value as float64 (%w) or as string (%w)", err, strUnmarshalErr)
		}
		floatVal, err = strconv.ParseFloat(strVal, 64) //nolint:gomnd
		if err != nil {
			return fmt.Errorf("string value should contain a scalar: %w", err)
		}
	}

	*f = Float64MaybeStringJSON(floatVal)
	return nil
}

// MarshalJSON return a Point as an array of value (to be compatible with v1 API)
// FIXME(maxime): to be removed when v2 endpoints are available
// Note: it is not used with jsoniter, encodePoints takes over
func (p *Point) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("[%v, %v]", int64(p.Ts), p.Value)), nil
}

// Serie holds a timeseries (w/ json serialization to DD API format)
type Serie struct {
	Name           string        `json:"metric"`
	Points         []Point       `json:"points"`
	Tags           []string      `json:"tags"`
	Host           string        `json:"host"`
	Device         string        `json:"device,omitempty"` // FIXME(olivier): remove as soon as the v1 API can handle `device` as a regular tag
	MType          APIMetricType `json:"type"`
	Interval       int64         `json:"interval"`
	SourceTypeName string        `json:"source_type_name,omitempty"`
	// ContextKey     ckey.ContextKey `json:"-"`
	// NameSuffix     string          `json:"-"`
}

// Series represents a list of Serie ready to be serialize
type Series []*Serie

// UnmarshalJSON is a custom unmarshaller for Point (used for testing)
func (p *Point) UnmarshalJSON(buf []byte) error {
	tmp := []interface{}{&p.Ts, &p.Value}
	wantLen := len(tmp)
	if err := json.Unmarshal(buf, &tmp); err != nil {
		return err
	}
	if len(tmp) != wantLen {
		return fmt.Errorf("wrong number of fields in Point: %d != %d", len(tmp), wantLen)
	}
	return nil
}
