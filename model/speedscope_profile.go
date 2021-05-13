// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package model

import (
	"encoding/json"
	"fmt"
	"github.com/elastic/apm-server/decoder"
	"io"
	"reflect"
)

type SpeedscopeProfileEvent struct {
	Metadata Metadata
	File     *SpeedscopeFile
}

type SpeedscopeFile struct {
	Schema *string `json:"$schema"`
	ActiveProfileIndex *float64 `json:"activeProfileIndex,omitempty"`
	Exporter *string `json:"exporter,omitempty"`
	Name *string `json:"name,omitempty"`
	// collection of EventedProfile or SampledProfile
	Profiles []Profile `json:"profiles"`
	Shared Shared `json:"shared"`
}

type Profile interface {
	isProfile()
}

type EventedProfile struct {
	EndValue float64 `json:"endValue"`
	Events []FrameEvent `json:"events"`
	Name string `json:"name"`
	StartValue float64 `json:"startValue"`
	Type ProfileType `json:"type"`
	Unit ValueUnit `json:"unit"`
}

func (e EventedProfile) isProfile() {}

type SampledProfile struct {
	EndValue float64 `json:"endValue"`
	Name string `json:"name"`
	Samples [][]float64 `json:"samples"`
	StartValue float64 `json:"startValue"`
	Type ProfileType `json:"type"`
	Unit ValueUnit `json:"unit"`
	Weights []float64 `json:"weights"`
}

func (e SampledProfile) isProfile() {}

type FrameEvent struct {
	At float64 `json:"at"`
	Frame float64 `json:"frame"`
	Type FrameEventType `json:"type"`
}

type Shared struct {
	Frames []Frame `json:"frames"`
}

type Frame struct {
	Col *float64 `json:"col,omitempty"`
	File *string `json:"file,omitempty"`
	Line *float64 `json:"line,omitempty"`
	Name string `json:"name"`
}

type FrameEventType string

const Close FrameEventType = "C"
const Open FrameEventType = "O"

var eventTypes = []interface{}{
	"C",
	"O",
}

type ProfileType string

const Evented ProfileType = "evented"
const Sampled ProfileType = "sampled"

var profileTypes = []interface{}{
	"evented",
	"sampled",
}

type ValueUnit string

const None ValueUnit = "none"
const Bytes ValueUnit = "bytes"
const Microseconds ValueUnit = "microseconds"
const Milliseconds ValueUnit = "milliseconds"
const Nanoseconds ValueUnit = "nanoseconds"
const Seconds ValueUnit = "seconds"

var valueUnits = []interface{}{
	"bytes",
	"microseconds",
	"milliseconds",
	"nanoseconds",
	"none",
	"seconds",
}

func (j *ValueUnit) UnmarshalJSON(b []byte) error {
	var v string
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	var ok bool
	for _, expected := range valueUnits {
		if reflect.DeepEqual(v, expected) {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("invalid value (expected one of %#v): %#v", valueUnits, v)
	}
	*j = ValueUnit(v)
	return nil
}

func (p *SpeedscopeFile) UnmarshalJSON(b []byte) error {
	var objMap map[string]*json.RawMessage
	err := json.Unmarshal(b, &objMap)
	if err != nil {
		return err
	}

	val, ok := objMap["$schema"]
	if !ok {
		return fmt.Errorf("field $schema is required")
	}
	var schema string
	err = json.Unmarshal(*val, &schema)
	if err != nil {
		return err
	}
	p.Schema = &schema

	val, ok = objMap["shared"]
	if !ok {
		return fmt.Errorf("field shared is required")
	}
	var shared Shared
	err = json.Unmarshal(*val, &shared)
	if err != nil {
		return err
	}
	p.Shared = shared

	// handle profiles union type
	val, ok = objMap["profiles"]
	if !ok {
		return fmt.Errorf("field profiles is required")
	}
	var profiles []*json.RawMessage
	err = json.Unmarshal(*val, &profiles)
	if err != nil {
		return err
	}

	p.Profiles = make([]Profile, len(profiles))
	var raw map[string]interface{}
	for index, rawMessage := range profiles {
		err = json.Unmarshal(*rawMessage, &raw)
		if err != nil {
			return err
		}
		if raw["type"] == "evented" {
			var e EventedProfile
			err := json.Unmarshal(*rawMessage, &e)
			if err != nil {
				return err
			}
			p.Profiles[index] = &e
		} else if raw["type"] == "sampled" {
			var s SampledProfile
			err := json.Unmarshal(*rawMessage, &s)
			if err != nil {
				return err
			}
			p.Profiles[index] = &s
		} else {
			return fmt.Errorf("unsupported type %#v", raw["type"])
		}
	}

	if val, ok := objMap["name"]; ok {
		var name string
		err = json.Unmarshal(*val, &name)
		if err != nil {
			return err
		}
		p.Name = &name
	}

	if val, ok := objMap["exporter"]; ok {
		var exporter string
		err = json.Unmarshal(*val, &exporter)
		if err != nil {
			return err
		}
		p.Exporter = &exporter
	}

	if val, ok := objMap["activeProfileIndex"]; ok {
		var activeProfileIndex float64
		err = json.Unmarshal(*val, &activeProfileIndex)
		if err != nil {
			return err
		}
		p.ActiveProfileIndex = &activeProfileIndex
	}

	return nil
}

func (e *SampledProfile) UnmarshalJSON(b []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if v, ok := raw["endValue"]; !ok || v == nil {
		return fmt.Errorf("field endValue: required")
	}
	if v, ok := raw["name"]; !ok || v == nil {
		return fmt.Errorf("field name: required")
	}
	if v, ok := raw["samples"]; !ok || v == nil {
		return fmt.Errorf("field samples: required")
	}
	if v, ok := raw["startValue"]; !ok || v == nil {
		return fmt.Errorf("field startValue: required")
	}
	if v, ok := raw["type"]; !ok || v == nil {
		return fmt.Errorf("field type: required")
	}
	if v, ok := raw["unit"]; !ok || v == nil {
		return fmt.Errorf("field unit: required")
	}
	if v, ok := raw["weights"]; !ok || v == nil {
		return fmt.Errorf("field weights: required")
	}
	type Plain SampledProfile
	var plain Plain
	if err := json.Unmarshal(b, &plain); err != nil {
		return err
	}
	*e = SampledProfile(plain)
	return nil
}

func (e *EventedProfile) UnmarshalJSON(b []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if v, ok := raw["endValue"]; !ok || v == nil {
		return fmt.Errorf("field endValue: required")
	}
	if v, ok := raw["events"]; !ok || v == nil {
		return fmt.Errorf("field events: required")
	}
	if v, ok := raw["name"]; !ok || v == nil {
		return fmt.Errorf("field name: required")
	}
	if v, ok := raw["startValue"]; !ok || v == nil {
		return fmt.Errorf("field startValue: required")
	}
	if v, ok := raw["type"]; !ok || v == nil {
		return fmt.Errorf("field type: required")
	}
	if v, ok := raw["unit"]; !ok || v == nil {
		return fmt.Errorf("field unit: required")
	}
	type Plain EventedProfile
	var plain Plain
	if err := json.Unmarshal(b, &plain); err != nil {
		return err
	}
	*e = EventedProfile(plain)
	return nil
}

func (j *FrameEventType) UnmarshalJSON(b []byte) error {
	var v string
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	var ok bool
	for _, expected := range eventTypes {
		if reflect.DeepEqual(v, expected) {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("invalid value (expected one of %#v): %#v", eventTypes, v)
	}
	*j = FrameEventType(v)
	return nil
}

func (j *Frame) UnmarshalJSON(b []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if v, ok := raw["name"]; !ok || v == nil {
		return fmt.Errorf("field name: required")
	}
	type Plain Frame
	var plain Plain
	if err := json.Unmarshal(b, &plain); err != nil {
		return err
	}
	*j = Frame(plain)
	return nil
}

func (j *FrameEvent) UnmarshalJSON(b []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if v, ok := raw["at"]; !ok || v == nil {
		return fmt.Errorf("field at: required")
	}
	if v, ok := raw["frame"]; !ok || v == nil {
		return fmt.Errorf("field frame: required")
	}
	if v, ok := raw["type"]; !ok || v == nil {
		return fmt.Errorf("field type: required")
	}
	type Plain FrameEvent
	var plain Plain
	if err := json.Unmarshal(b, &plain); err != nil {
		return err
	}
	*j = FrameEvent(plain)
	return nil
}

func (j *Shared) UnmarshalJSON(b []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if v, ok := raw["frames"]; !ok || v == nil {
		return fmt.Errorf("field frames: required")
	}
	type Plain Shared
	var plain Plain
	if err := json.Unmarshal(b, &plain); err != nil {
		return err
	}
	*j = Shared(plain)
	return nil
}

func (j *ProfileType) UnmarshalJSON(b []byte) error {
	var v string
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	var ok bool
	for _, expected := range profileTypes {
		if reflect.DeepEqual(v, expected) {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("invalid value (expected one of %#v): %#v", profileTypes, v)
	}
	*j = ProfileType(v)
	return nil
}

func ParseSpeedscopeFile(r io.Reader) (*SpeedscopeFile, error) {
	var sf SpeedscopeFile

	dec := decoder.NewJSONDecoder(r)
	if err := dec.Decode(&sf); err != nil {
		return nil, err
	}

	return &sf, nil
}
