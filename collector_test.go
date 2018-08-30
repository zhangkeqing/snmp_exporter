// Copyright 2018 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"reflect"
	"regexp"
	"testing"

	"github.com/prometheus/client_model/go"
	"github.com/soniah/gosnmp"

	"github.com/prometheus/snmp_exporter/config"
)

func TestPduToSample(t *testing.T) {

	cases := []struct {
		pdu             *gosnmp.SnmpPDU
		indexOids       []int
		metric          *config.Metric
		oidToPdu        map[string]gosnmp.SnmpPDU
		expectedMetrics map[string]string
		shouldErr       bool
	}{
		{
			pdu: &gosnmp.SnmpPDU{
				Name:  "1.1.1.1.1",
				Value: "SomeStringValue",
			},
			indexOids: []int{},
			metric: &config.Metric{
				Name: "TestMetricName",
				Oid:  "1.1.1.1.1",
				Help: "HelpText",
				RegexpExtracts: map[string][]config.RegexpExtract{
					"Extension": []config.RegexpExtract{
						{
							Regex: config.Regexp{
								regexp.MustCompile(".*"),
							},
							Value: "5",
						},
					},
				},
			},
			oidToPdu: make(map[string]gosnmp.SnmpPDU),
			expectedMetrics: map[string]string{
				`gauge:<value:5 > `: `Desc{fqName: "TestMetricNameExtension", help: "HelpText (regex extracted)", constLabels: {}, variableLabels: []}`,
			},
		},
		{
			pdu: &gosnmp.SnmpPDU{
				Name:  "1.1.1.1.1",
				Value: "SomeStringValue",
			},
			indexOids: []int{},
			metric: &config.Metric{
				Name: "TestMetricName",
				Oid:  "1.1.1.1.1",
				Help: "HelpText",
				RegexpExtracts: map[string][]config.RegexpExtract{
					"Extension": []config.RegexpExtract{
						{
							Regex: config.Regexp{
								regexp.MustCompile(".*"),
							},
							Value: "",
						},
					},
				},
			},
			expectedMetrics: map[string]string{},
		},
		{
			pdu: &gosnmp.SnmpPDU{
				Name:  "1.1.1.1.1",
				Value: "SomeStringValue",
			},
			indexOids: []int{},
			metric: &config.Metric{
				Name: "TestMetricName",
				Oid:  "1.1.1.1.1",
				Help: "HelpText",
				RegexpExtracts: map[string][]config.RegexpExtract{
					"Extension": []config.RegexpExtract{
						{
							Regex: config.Regexp{
								regexp.MustCompile("(will_not_match)"),
							},
							Value: "",
						},
					},
				},
			},
			expectedMetrics: map[string]string{},
		},
		{
			pdu: &gosnmp.SnmpPDU{
				Name:  "1.1.1.1.1",
				Value: 2,
			},
			indexOids: []int{},
			metric: &config.Metric{
				Name: "TestMetricName",
				Oid:  "1.1.1.1.1",
				Help: "HelpText",
				RegexpExtracts: map[string][]config.RegexpExtract{
					"Status": []config.RegexpExtract{
						{
							Regex: config.Regexp{
								regexp.MustCompile(".*"),
							},
							Value: "5",
						},
					},
				},
			},
			expectedMetrics: map[string]string{
				`gauge:<value:5 > `: `Desc{fqName: "TestMetricNameStatus", help: "HelpText (regex extracted)", constLabels: {}, variableLabels: []}`,
			},
		},
		{
			pdu: &gosnmp.SnmpPDU{
				Name:  "1.1.1.1.1",
				Value: "Test value 4.42 123 999",
			},
			indexOids: []int{},
			metric: &config.Metric{
				Name: "TestMetricName",
				Oid:  "1.1.1.1.1",
				Help: "HelpText",
				RegexpExtracts: map[string][]config.RegexpExtract{
					"Blank": []config.RegexpExtract{
						{
							Regex: config.Regexp{
								regexp.MustCompile("XXXX"),
							},
							Value: "4",
						},
					},
					"Extension": []config.RegexpExtract{
						{
							Regex: config.Regexp{
								regexp.MustCompile(".*"),
							},
							Value: "5",
						},
					},
					"MultipleRegexes": []config.RegexpExtract{
						{
							Regex: config.Regexp{
								regexp.MustCompile("XXXX"),
							},
							Value: "123",
						},
						{
							Regex: config.Regexp{
								regexp.MustCompile("123"),
							},
							Value: "999",
						},
						{
							Regex: config.Regexp{
								regexp.MustCompile(".*"),
							},
							Value: "777",
						},
					},
					"Template": []config.RegexpExtract{
						{
							Regex: config.Regexp{
								regexp.MustCompile("([0-9].[0-9]+)"),
							},
							Value: "$1",
						},
					},
				},
			},
			oidToPdu: make(map[string]gosnmp.SnmpPDU),
			expectedMetrics: map[string]string{
				`gauge:<value:5 > `:    `Desc{fqName: "TestMetricNameExtension", help: "HelpText (regex extracted)", constLabels: {}, variableLabels: []}`,
				`gauge:<value:999 > `:  `Desc{fqName: "TestMetricNameMultipleRegexes", help: "HelpText (regex extracted)", constLabels: {}, variableLabels: []}`,
				`gauge:<value:4.42 > `: `Desc{fqName: "TestMetricNameTemplate", help: "HelpText (regex extracted)", constLabels: {}, variableLabels: []}`,
			},
		},
		{
			pdu: &gosnmp.SnmpPDU{
				Name:  "1.1.1.1.1",
				Type:  gosnmp.Integer,
				Value: 2,
			},
			indexOids: []int{},
			metric: &config.Metric{
				Name: "test_metric",
				Oid:  "1.1.1.1.1",
				Type: "counter",
				Help: "Help string",
			},
			oidToPdu:        make(map[string]gosnmp.SnmpPDU),
			expectedMetrics: map[string]string{"counter:<value:2 > ": `Desc{fqName: "test_metric", help: "Help string", constLabels: {}, variableLabels: []}`},
		},
		{
			pdu: &gosnmp.SnmpPDU{
				Name:  "1.1.1.1.1",
				Type:  gosnmp.Integer,
				Value: 2,
			},
			indexOids: []int{},
			metric: &config.Metric{
				Name: "test_metric",
				Oid:  "1.1.1.1.1",
				Type: "gauge",
				Help: "Help string",
			},
			oidToPdu:        make(map[string]gosnmp.SnmpPDU),
			expectedMetrics: map[string]string{"gauge:<value:2 > ": `Desc{fqName: "test_metric", help: "Help string", constLabels: {}, variableLabels: []}`},
		},
		{
			pdu: &gosnmp.SnmpPDU{
				Name:  "1.1.1.1.1",
				Type:  gosnmp.Integer,
				Value: -2,
			},
			indexOids: []int{},
			metric: &config.Metric{
				Name: "test_metric",
				Oid:  "1.1.1.1.1",
				Help: "Help string",
			},
			oidToPdu:        make(map[string]gosnmp.SnmpPDU),
			expectedMetrics: map[string]string{`label:<name:"test_metric" value:"-2" > gauge:<value:1 > `: `Desc{fqName: "test_metric", help: "Help string", constLabels: {}, variableLabels: [test_metric]}`},
		},
		{
			pdu: &gosnmp.SnmpPDU{
				Name:  "1.1.1.1.1",
				Type:  gosnmp.OpaqueFloat,
				Value: float32(3.0),
			},
			indexOids: []int{},
			metric: &config.Metric{
				Name: "test_metric",
				Oid:  "1.1.1.1.1",
				Type: "gauge",
				Help: "Help string",
			},
			oidToPdu:        make(map[string]gosnmp.SnmpPDU),
			expectedMetrics: map[string]string{"gauge:<value:3 > ": `Desc{fqName: "test_metric", help: "Help string", constLabels: {}, variableLabels: []}`},
		},
		{
			pdu: &gosnmp.SnmpPDU{
				Name:  "1.1.1.1.1",
				Type:  gosnmp.OpaqueDouble,
				Value: float64(3.0),
			},
			indexOids: []int{},
			metric: &config.Metric{
				Name: "test_metric",
				Oid:  "1.1.1.1.1",
				Type: "gauge",
				Help: "Help string",
			},
			oidToPdu:        make(map[string]gosnmp.SnmpPDU),
			expectedMetrics: map[string]string{"gauge:<value:3 > ": `Desc{fqName: "test_metric", help: "Help string", constLabels: {}, variableLabels: []}`},
		},
		{
			pdu: &gosnmp.SnmpPDU{
				Name:  "1.1.1.1.1",
				Type:  gosnmp.Integer,
				Value: 3,
			},
			indexOids: []int{2, 65, 65},
			metric: &config.Metric{
				Name:    "test_metric",
				Oid:     "1.1.1.1.1",
				Type:    "gauge",
				Help:    "Help string",
				Indexes: []*config.Index{{Labelname: "foo", Type: "DisplayString"}},
			},
			oidToPdu:        make(map[string]gosnmp.SnmpPDU),
			expectedMetrics: map[string]string{`label:<name:"foo" value:"AA" > gauge:<value:3 > `: `Desc{fqName: "test_metric", help: "Help string", constLabels: {}, variableLabels: [foo]}`},
		},
		{
			pdu: &gosnmp.SnmpPDU{
				Name:  "1.1.1.1.1",
				Type:  gosnmp.Integer,
				Value: 3,
			},
			indexOids: []int{2, 65, 255},
			metric: &config.Metric{
				Name:    "test_metric",
				Oid:     "1.1.1.1.1",
				Type:    "gauge",
				Help:    "Help string",
				Indexes: []*config.Index{{Labelname: "foo", Type: "DisplayString"}},
			},
			oidToPdu:  make(map[string]gosnmp.SnmpPDU),
			shouldErr: true, // Invalid ASCII/UTF-8 string.
		},
		{
			pdu: &gosnmp.SnmpPDU{
				Name:  "1.1.1.1.1",
				Type:  gosnmp.Integer,
				Value: 3,
			},
			indexOids: []int{2, 65, 255},
			metric: &config.Metric{
				Name:    "test_metric",
				Oid:     "1.1.1.1.1",
				Type:    "gauge",
				Help:    "Help string",
				Indexes: []*config.Index{{Labelname: "foo", Type: "DisplayString"}},
				RegexpExtracts: map[string][]config.RegexpExtract{
					"": []config.RegexpExtract{{Value: "1", Regex: config.Regexp{regexp.MustCompile(".*")}}},
				},
			},
			oidToPdu:  make(map[string]gosnmp.SnmpPDU),
			shouldErr: true, // Invalid ASCII/UTF-8 string.
		},
	}

	for i, c := range cases {
		metrics := pduToSamples(c.indexOids, c.pdu, c.metric, c.oidToPdu)
		if len(metrics) != len(c.expectedMetrics) && !c.shouldErr {
			t.Fatalf("Unexpected number of metrics returned for case %v: want %v, got %v", i, len(c.expectedMetrics), len(metrics))
		}
		metric := &io_prometheus_client.Metric{}
		errHappened := false
		for _, m := range metrics {
			err := m.Write(metric)
			if err != nil {
				if c.shouldErr {
					errHappened = true
					continue
				} else {
					t.Fatalf("Error writing metric: %v", err)
				}
			}
			if _, ok := c.expectedMetrics[metric.String()]; !ok {
				t.Fatalf("Unexpected metric: got %v", metric.String())
			}
			if c.expectedMetrics[metric.String()] != m.Desc().String() {
				t.Fatalf("Unexpected metric: got %v , want %v", m.Desc().String(), c.expectedMetrics[metric.String()])
			}
		}
		if !errHappened && c.shouldErr {
			t.Fatalf("Was expecting error, but none returned.")
		}
	}
}

func TestGetPduValue(t *testing.T) {
	pdu := &gosnmp.SnmpPDU{
		Value: uint64(1 << 63),
		Type:  gosnmp.Counter64,
	}
	value := getPduValue(pdu)
	if value <= 0 {
		t.Fatalf("Got negative value for PDU value type Counter64: %v", value)
	}
}

func TestOidToList(t *testing.T) {
	cases := []struct {
		oid    string
		result []int
	}{
		{
			oid:    "1",
			result: []int{1},
		},
		{
			oid:    "1.2.3.4",
			result: []int{1, 2, 3, 4},
		},
	}
	for _, c := range cases {
		got := oidToList(c.oid)
		if !reflect.DeepEqual(got, c.result) {
			t.Errorf("oidToList(%v): got %v, want %v", c.oid, got, c.result)
		}
	}
}

func TestSplitOid(t *testing.T) {
	cases := []struct {
		oid        []int
		count      int
		resultHead []int
		resultTail []int
	}{
		{
			oid:        []int{1, 2, 3, 4},
			count:      2,
			resultHead: []int{1, 2},
			resultTail: []int{3, 4},
		},
		{
			oid:        []int{1, 2},
			count:      4,
			resultHead: []int{1, 2, 0, 0},
			resultTail: []int{},
		},
		{
			oid:        []int{},
			count:      2,
			resultHead: []int{0, 0},
			resultTail: []int{},
		},
	}
	for _, c := range cases {
		head, tail := splitOid(c.oid, c.count)
		if !reflect.DeepEqual(head, c.resultHead) || !reflect.DeepEqual(tail, c.resultTail) {
			t.Errorf("splitOid(%v, %d): got [%v, %v], want [%v, %v]", c.oid, c.count, head, tail, c.resultHead, c.resultTail)
		}
	}
}

func TestPduValueAsString(t *testing.T) {
	cases := []struct {
		pdu    *gosnmp.SnmpPDU
		typ    string
		result string
	}{
		{
			pdu:    &gosnmp.SnmpPDU{Value: int(-1)},
			result: "-1",
		},
		{
			pdu:    &gosnmp.SnmpPDU{Value: uint(1)},
			result: "1",
		},
		{
			pdu:    &gosnmp.SnmpPDU{Value: uint64(1)},
			result: "1",
		},
		{
			pdu:    &gosnmp.SnmpPDU{Value: ".1.2.3.4", Type: gosnmp.ObjectIdentifier},
			result: "1.2.3.4",
		},
		{
			pdu:    &gosnmp.SnmpPDU{Value: "1.2.3.4", Type: gosnmp.IPAddress},
			result: "1.2.3.4",
		},
		{
			pdu:    &gosnmp.SnmpPDU{Value: []byte{}},
			result: "",
		},
		{
			pdu:    &gosnmp.SnmpPDU{Value: []byte{65, 66}},
			typ:    "DisplayString",
			result: "AB",
		},
		{
			pdu:    &gosnmp.SnmpPDU{Value: []byte{127, 128, 255, 0}},
			result: "0x7F80FF00",
		},
		{
			pdu:    &gosnmp.SnmpPDU{Value: []byte{127, 128, 255, 0}},
			typ:    "OctetString",
			result: "0x7F80FF00",
		},
		{
			pdu:    &gosnmp.SnmpPDU{Value: []byte{1, 2, 3, 4}},
			typ:    "IpAddr",
			result: "1.2.3.4",
		},
		{
			pdu:    &gosnmp.SnmpPDU{Value: nil},
			result: "",
		},
		{
			pdu:    &gosnmp.SnmpPDU{Value: float32(10.1), Type: gosnmp.OpaqueFloat},
			result: "10.1",
		},
		{
			pdu:    &gosnmp.SnmpPDU{Value: 10.1, Type: gosnmp.OpaqueDouble},
			result: "10.1",
		},
	}
	for _, c := range cases {
		got := pduValueAsString(c.pdu, c.typ)
		if !reflect.DeepEqual(got, c.result) {
			t.Errorf("pduValueAsString(%v, %q): got %q, want %q", c.pdu, c.typ, got, c.result)
		}
	}
}

func TestIndexesToLabels(t *testing.T) {
	cases := []struct {
		oid      []int
		metric   config.Metric
		oidToPdu map[string]gosnmp.SnmpPDU
		result   map[string]string
	}{
		{
			oid:      []int{},
			metric:   config.Metric{},
			oidToPdu: map[string]gosnmp.SnmpPDU{},
			result:   map[string]string{},
		},
		{
			oid:      []int{4},
			metric:   config.Metric{Indexes: []*config.Index{{Labelname: "l", Type: "gauge"}}},
			oidToPdu: map[string]gosnmp.SnmpPDU{},
			result:   map[string]string{"l": "4"},
		},
		{
			oid: []int{3, 4},
			metric: config.Metric{
				Indexes: []*config.Index{{Labelname: "a", Type: "gauge"}, {Labelname: "b", Type: "gauge"}},
				Lookups: []*config.Lookup{{Labels: []string{"a", "b"}, Labelname: "l", Oid: "1.2"}},
			},
			oidToPdu: map[string]gosnmp.SnmpPDU{"1.2.3.4": gosnmp.SnmpPDU{Value: "eth0"}},
			result:   map[string]string{"a": "3", "b": "4", "l": "eth0"},
		},
		{
			oid: []int{4},
			metric: config.Metric{
				Indexes: []*config.Index{{Labelname: "l", Type: "gauge"}},
				Lookups: []*config.Lookup{{Labels: []string{"l"}, Labelname: "l", Oid: "1.2.3"}},
			},
			oidToPdu: map[string]gosnmp.SnmpPDU{"1.2.3.4": gosnmp.SnmpPDU{Value: "eth0"}},
			result:   map[string]string{"l": "eth0"},
		},
		{
			oid: []int{4},
			metric: config.Metric{
				Indexes: []*config.Index{{Labelname: "l", Type: "gauge"}},
				Lookups: []*config.Lookup{{Labels: []string{"l"}, Labelname: "l", Oid: "1.2.3", Type: "IpAddr"}},
			},
			oidToPdu: map[string]gosnmp.SnmpPDU{"1.2.3.4": gosnmp.SnmpPDU{Value: []byte{5, 6, 7, 8}}},
			result:   map[string]string{"l": "5.6.7.8"},
		},
		{
			oid: []int{4},
			metric: config.Metric{
				Indexes: []*config.Index{{Labelname: "l", Type: "gauge"}},
				Lookups: []*config.Lookup{{Labels: []string{"l"}, Labelname: "l", Oid: "1.2.3"}},
			},
			oidToPdu: map[string]gosnmp.SnmpPDU{"1.2.3.4": gosnmp.SnmpPDU{Value: []byte{5, 6, 7, 8}}},
			result:   map[string]string{"l": "0x05060708"},
		},
		{
			oid: []int{4},
			metric: config.Metric{
				Indexes: []*config.Index{{Labelname: "l", Type: "gauge"}},
				Lookups: []*config.Lookup{{Labels: []string{"l"}, Labelname: "l", Oid: "1.2.3"}},
			},
			oidToPdu: map[string]gosnmp.SnmpPDU{},
			result:   map[string]string{"l": ""},
		},
		{
			oid:      []int{},
			metric:   config.Metric{Indexes: []*config.Index{{Labelname: "l", Type: "gauge"}}},
			oidToPdu: map[string]gosnmp.SnmpPDU{},
			result:   map[string]string{"l": "0"},
		},
		{
			oid:      []int{1, 255, 0, 0, 0, 16},
			metric:   config.Metric{Indexes: []*config.Index{{Labelname: "l", Type: "PhysAddress48"}}},
			oidToPdu: map[string]gosnmp.SnmpPDU{},
			result:   map[string]string{"l": "01:FF:00:00:00:10"},
		},
		{
			oid:      []int{3, 65, 32, 255},
			metric:   config.Metric{Indexes: []*config.Index{{Labelname: "l", Type: "OctetString"}}},
			oidToPdu: map[string]gosnmp.SnmpPDU{},
			result:   map[string]string{"l": "0x4120FF"},
		},
		{
			oid:      []int{65, 32, 255},
			metric:   config.Metric{Indexes: []*config.Index{{Labelname: "l", Type: "OctetString", FixedSize: 3}}},
			oidToPdu: map[string]gosnmp.SnmpPDU{},
			result:   map[string]string{"l": "0x4120FF"},
		},
		{
			oid:      []int{2, 65, 32},
			metric:   config.Metric{Indexes: []*config.Index{{Labelname: "l", Type: "DisplayString"}}},
			oidToPdu: map[string]gosnmp.SnmpPDU{},
			result:   map[string]string{"l": "A "},
		},
		{
			oid:      []int{65, 32},
			metric:   config.Metric{Indexes: []*config.Index{{Labelname: "l", Type: "DisplayString", FixedSize: 2}}},
			oidToPdu: map[string]gosnmp.SnmpPDU{},
			result:   map[string]string{"l": "A "},
		},
		{
			oid: []int{3, 65, 32, 255},
			metric: config.Metric{
				Indexes: []*config.Index{{Labelname: "l", Type: "OctetString"}},
				Lookups: []*config.Lookup{{Labels: []string{"l"}, Labelname: "l", Oid: "1"}},
			},
			oidToPdu: map[string]gosnmp.SnmpPDU{"1.3.65.32.255": gosnmp.SnmpPDU{Value: "octet"}},
			result:   map[string]string{"l": "octet"},
		},
		{
			oid:      []int{192, 168, 1, 2},
			metric:   config.Metric{Indexes: []*config.Index{{Labelname: "l", Type: "IpAddr"}}},
			oidToPdu: map[string]gosnmp.SnmpPDU{},
			result:   map[string]string{"l": "192.168.1.2"},
		},
		{
			oid: []int{0, 1, 2, 3, 4, 16, 42},
			metric: config.Metric{
				Indexes: []*config.Index{
					{Labelname: "a", Type: "InetAddressType"},
					{Labelname: "b", Type: "InetAddressType"},
					{Labelname: "c", Type: "InetAddressType"},
					{Labelname: "d", Type: "InetAddressType"},
					{Labelname: "e", Type: "InetAddressType"},
					{Labelname: "f", Type: "InetAddressType"},
					{Labelname: "g", Type: "InetAddressType"},
				},
			},
			oidToPdu: map[string]gosnmp.SnmpPDU{},
			result: map[string]string{
				"a": "unknown",
				"b": "ipv4",
				"c": "ipv6",
				"d": "ipv4z",
				"e": "ipv6z",
				"f": "dns",
				"g": "42",
			},
		},
	}
	for _, c := range cases {
		got := indexesToLabels(c.oid, &c.metric, c.oidToPdu)
		if !reflect.DeepEqual(got, c.result) {
			t.Errorf("oidToList(%v, %v, %v): got %v, want %v", c.oid, c.metric, c.oidToPdu, got, c.result)
		}
	}
}
