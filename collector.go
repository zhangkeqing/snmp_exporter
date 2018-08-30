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
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/soniah/gosnmp"

	"github.com/prometheus/snmp_exporter/config"
)

var (
	snmpUnexpectedPduType = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "snmp_unexpected_pdu_type_total",
			Help: "Unexpected Go types in a PDU.",
		},
	)
)

func init() {
	prometheus.MustRegister(snmpUnexpectedPduType)
}

func oidToList(oid string) []int {
	result := []int{}
	for _, x := range strings.Split(oid, ".") {
		o, _ := strconv.Atoi(x)
		result = append(result, o)
	}
	return result
}

func ScrapeTarget(target string, config *config.Module) ([]gosnmp.SnmpPDU, error) {
	// Set the options.
	snmp := gosnmp.GoSNMP{}
	snmp.MaxRepetitions = config.WalkParams.MaxRepetitions
	// User specifies timeout of each retry attempt but GoSNMP expects total timeout for all attemtps.
	snmp.Retries = config.WalkParams.Retries
	snmp.Timeout = config.WalkParams.Timeout * time.Duration(snmp.Retries)

	snmp.Target = target
	snmp.Port = 161
	if host, port, err := net.SplitHostPort(target); err == nil {
		snmp.Target = host
		p, err := strconv.Atoi(port)
		if err != nil {
			return nil, fmt.Errorf("Error converting port number to int for target %s: %s", target, err)
		}
		snmp.Port = uint16(p)
	}

	// Configure auth.
	config.WalkParams.ConfigureSNMP(&snmp)

	// Do the actual walk.
	err := snmp.Connect()
	if err != nil {
		return nil, fmt.Errorf("Error connecting to target %s: %s", target, err)
	}
	defer snmp.Conn.Close()

	result := []gosnmp.SnmpPDU{}
	getOids := config.Get
	maxOids := int(config.WalkParams.MaxRepetitions)
	// Max Repetition can be 0, maxOids cannot. SNMPv1 can only report one OID error per call.
	if maxOids == 0 || snmp.Version == gosnmp.Version1 {
		maxOids = 1
	}
	for len(getOids) > 0 {
		oids := len(getOids)
		if oids > maxOids {
			oids = maxOids
		}

		log.Debugf("Getting %d OIDs from target %q", oids, snmp.Target)
		getStart := time.Now()
		packet, err := snmp.Get(getOids[:oids])
		if err != nil {
			return nil, fmt.Errorf("Error getting target %s: %s", snmp.Target, err)
		}
		log.Debugf("Get of %d OIDs completed in %s", oids, time.Since(getStart))
		// SNMPv1 will return packet error for unsupported OIDs.
		if packet.Error == gosnmp.NoSuchName && snmp.Version == gosnmp.Version1 {
			log.Debugf("OID %s not supported by target %s", getOids[0], snmp.Target)
			getOids = getOids[oids:]
			continue
		}
		// Response received with errors.
		// TODO: "stringify" gosnmp errors instead of showing error code.
		if packet.Error != gosnmp.NoError {
			return nil, fmt.Errorf("Error reported by target %s: Error Status %d", snmp.Target, packet.Error)
		}
		for _, v := range packet.Variables {
			if v.Type == gosnmp.NoSuchObject || v.Type == gosnmp.NoSuchInstance {
				log.Debugf("OID %s not supported by target %s", v.Name, snmp.Target)
				continue
			}
			result = append(result, v)
		}
		getOids = getOids[oids:]
	}

	for _, subtree := range config.Walk {
		var pdus []gosnmp.SnmpPDU
		log.Debugf("Walking target %q subtree %q", snmp.Target, subtree)
		walkStart := time.Now()
		if snmp.Version == gosnmp.Version1 {
			pdus, err = snmp.WalkAll(subtree)
		} else {
			pdus, err = snmp.BulkWalkAll(subtree)
		}
		if err != nil {
			return nil, fmt.Errorf("Error walking target %s: %s", snmp.Target, err)
		}
		log.Debugf("Walk of target %q subtree %q completed in %s", snmp.Target, subtree, time.Since(walkStart))

		result = append(result, pdus...)
	}
	return result, nil
}

type MetricNode struct {
	metric *config.Metric

	children map[int]*MetricNode
}

// Build a tree of metrics from the config, for fast lookup when there's lots of them.
func buildMetricTree(metrics []*config.Metric) *MetricNode {
	metricTree := &MetricNode{children: map[int]*MetricNode{}}
	for _, metric := range metrics {
		head := metricTree
		for _, o := range oidToList(metric.Oid) {
			_, ok := head.children[o]
			if !ok {
				head.children[o] = &MetricNode{children: map[int]*MetricNode{}}
			}
			head = head.children[o]
		}
		head.metric = metric
	}
	return metricTree
}

type collector struct {
	target string
	module *config.Module
}

// Describe implements Prometheus.Collector.
func (c collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- prometheus.NewDesc("dummy", "dummy", nil, nil)
}

// Collect implements Prometheus.Collector.
func (c collector) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()
	pdus, err := ScrapeTarget(c.target, c.module)
	if err != nil {
		log.Infof("Error scraping target %s: %s", c.target, err)
		ch <- prometheus.NewInvalidMetric(prometheus.NewDesc("snmp_error", "Error scraping target", nil, nil), err)
		return
	}
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("snmp_scrape_walk_duration_seconds", "Time SNMP walk/bulkwalk took.", nil, nil),
		prometheus.GaugeValue,
		float64(time.Since(start).Seconds()))
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("snmp_scrape_pdus_returned", "PDUs returned from walk.", nil, nil),
		prometheus.GaugeValue,
		float64(len(pdus)))
	oidToPdu := make(map[string]gosnmp.SnmpPDU, len(pdus))
	for _, pdu := range pdus {
		oidToPdu[pdu.Name[1:]] = pdu
	}

	metricTree := buildMetricTree(c.module.Metrics)
	// Look for metrics that match each pdu.
PduLoop:
	for oid, pdu := range oidToPdu {
		head := metricTree
		oidList := oidToList(oid)
		for i, o := range oidList {
			var ok bool
			head, ok = head.children[o]
			if !ok {
				continue PduLoop
			}
			if head.metric != nil {
				// Found a match.
				samples := pduToSamples(oidList[i+1:], &pdu, head.metric, oidToPdu)
				for _, sample := range samples {
					ch <- sample
				}
				break
			}
		}
	}
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("snmp_scrape_duration_seconds", "Total SNMP time scrape took (walk and processing).", nil, nil),
		prometheus.GaugeValue,
		float64(time.Since(start).Seconds()))
}

func getPduValue(pdu *gosnmp.SnmpPDU) float64 {
	switch pdu.Type {
	case gosnmp.Counter64:
		return float64(gosnmp.ToBigInt(pdu.Value).Uint64())
	case gosnmp.OpaqueFloat:
		return float64(pdu.Value.(float32))
	case gosnmp.OpaqueDouble:
		return pdu.Value.(float64)
	default:
		return float64(gosnmp.ToBigInt(pdu.Value).Int64())
	}
}

func pduToSamples(indexOids []int, pdu *gosnmp.SnmpPDU, metric *config.Metric, oidToPdu map[string]gosnmp.SnmpPDU) []prometheus.Metric {
	// The part of the OID that is the indexes.
	labels := indexesToLabels(indexOids, metric, oidToPdu)

	value := getPduValue(pdu)
	t := prometheus.UntypedValue

	labelnames := make([]string, 0, len(labels)+1)
	labelvalues := make([]string, 0, len(labels)+1)
	for k, v := range labels {
		labelnames = append(labelnames, k)
		labelvalues = append(labelvalues, v)
	}

	switch metric.Type {
	case "counter":
		t = prometheus.CounterValue
	case "gauge":
		t = prometheus.GaugeValue
	case "Float", "Double":
		t = prometheus.GaugeValue
	default:
		// It's some form of string.
		t = prometheus.GaugeValue
		value = 1.0
		if len(metric.RegexpExtracts) > 0 {
			return applyRegexExtracts(metric, pduValueAsString(pdu, metric.Type), labelnames, labelvalues)
		}
		// For strings we put the value as a label with the same name as the metric.
		// If the name is already an index, we do not need to set it again.
		if _, ok := labels[metric.Name]; !ok {
			labelnames = append(labelnames, metric.Name)
			labelvalues = append(labelvalues, pduValueAsString(pdu, metric.Type))
		}
	}

	sample, err := prometheus.NewConstMetric(prometheus.NewDesc(metric.Name, metric.Help, labelnames, nil),
		t, value, labelvalues...)
	if err != nil {
		sample = prometheus.NewInvalidMetric(prometheus.NewDesc("snmp_error", "Error calling NewConstMetric", nil, nil),
			fmt.Errorf("Error for metric %s with labels %v from indexOids %v: %v", metric.Name, labelvalues, indexOids, err))
	}

	return []prometheus.Metric{sample}
}

func applyRegexExtracts(metric *config.Metric, pduValue string, labelnames, labelvalues []string) []prometheus.Metric {
	results := []prometheus.Metric{}
	for name, strMetricSlice := range metric.RegexpExtracts {
		for _, strMetric := range strMetricSlice {
			indexes := strMetric.Regex.FindStringSubmatchIndex(pduValue)
			if indexes == nil {
				log.Debugf("No match found for regexp: %v against value: %v for metric %v", strMetric.Regex.String(), pduValue, metric.Name)
				continue
			}
			res := strMetric.Regex.ExpandString([]byte{}, strMetric.Value, pduValue, indexes)
			v, err := strconv.ParseFloat(string(res), 64)
			if err != nil {
				log.Debugf("Error parsing float64 from value: %v for metric: %v", res, metric.Name)
				continue
			}
			newMetric, err := prometheus.NewConstMetric(prometheus.NewDesc(metric.Name+name, metric.Help+" (regex extracted)", labelnames, nil),
				prometheus.GaugeValue, v, labelvalues...)
			if err != nil {
				newMetric = prometheus.NewInvalidMetric(prometheus.NewDesc("snmp_error", "Error calling NewConstMetric for regex_extract", nil, nil),
					fmt.Errorf("Error for metric %s with labels %v: %v", metric.Name+name, labelvalues, err))
			}
			results = append(results, newMetric)
			break
		}
	}
	return results
}

// Right pad oid with zeros, and split at the given point.
// Some routers exclude trailing 0s in responses.
func splitOid(oid []int, count int) ([]int, []int) {
	head := make([]int, count)
	tail := []int{}
	for i, v := range oid {
		if i < count {
			head[i] = v
		} else {
			tail = append(tail, v)
		}
	}
	return head, tail
}

// This mirrors decodeValue in gosnmp's helper.go.
func pduValueAsString(pdu *gosnmp.SnmpPDU, typ string) string {
	switch pdu.Value.(type) {
	case int:
		return strconv.Itoa(pdu.Value.(int))
	case uint:
		return strconv.FormatUint(uint64(pdu.Value.(uint)), 10)
	case uint64:
		return strconv.FormatUint(pdu.Value.(uint64), 10)
	case float32:
		return strconv.FormatFloat(float64(pdu.Value.(float32)), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(pdu.Value.(float64), 'f', -1, 64)
	case string:
		if pdu.Type == gosnmp.ObjectIdentifier {
			// Trim leading period.
			return pdu.Value.(string)[1:]
		}
		// DisplayString.
		return pdu.Value.(string)
	case []byte:
		if typ == "" {
			typ = "OctetString"
		}
		// Reuse the OID index parsing code.
		parts := make([]int, len(pdu.Value.([]byte)))
		for i, o := range pdu.Value.([]byte) {
			parts[i] = int(o)
		}
		if typ == "OctetString" || typ == "DisplayString" {
			// Prepend the length, as it is explicit in an index.
			parts = append([]int{len(pdu.Value.([]byte))}, parts...)
		}
		str, _, _ := indexOidsAsString(parts, typ, 0)
		return str
	case nil:
		return ""
	default:
		// This shouldn't happen.
		log.Infof("Got PDU with unexpected type: Name: %s Value: '%s', Go Type: %T SNMP Type: %s", pdu.Name, pdu.Value, pdu.Value, pdu.Type)
		snmpUnexpectedPduType.Inc()
		return fmt.Sprintf("%s", pdu.Value)
	}
}

// Convert oids to a string index value.
//
// Returns the string, the oids that were used and the oids left over.
func indexOidsAsString(indexOids []int, typ string, fixedSize int) (string, []int, []int) {
	switch typ {
	case "Integer32", "Integer", "gauge", "counter":
		// Extract the oid for this index, and keep the remainder for the next index.
		subOid, indexOids := splitOid(indexOids, 1)
		return fmt.Sprintf("%d", subOid[0]), subOid, indexOids
	case "PhysAddress48":
		subOid, indexOids := splitOid(indexOids, 6)
		parts := make([]string, 6)
		for i, o := range subOid {
			parts[i] = fmt.Sprintf("%02X", o)
		}
		return strings.Join(parts, ":"), subOid, indexOids
	case "OctetString":
		var subOid []int
		// The length of fixed size indexes come from the MIB.
		// For varying size, we read it from the first oid.
		length := fixedSize
		if length == 0 {
			subOid, indexOids = splitOid(indexOids, 1)
			length = subOid[0]
		}
		content, indexOids := splitOid(indexOids, length)
		subOid = append(subOid, content...)
		parts := make([]byte, length)
		for i, o := range content {
			parts[i] = byte(o)
		}
		if len(parts) == 0 {
			return "", subOid, indexOids
		} else {
			return fmt.Sprintf("0x%X", string(parts)), subOid, indexOids
		}
	case "DisplayString":
		var subOid []int
		length := fixedSize
		if length == 0 {
			subOid, indexOids = splitOid(indexOids, 1)
			length = subOid[0]
		}
		content, indexOids := splitOid(indexOids, length)
		subOid = append(subOid, content...)
		parts := make([]byte, length)
		for i, o := range content {
			parts[i] = byte(o)
		}
		// ASCII, so can convert staight to utf-8.
		return string(parts), subOid, indexOids
	case "IpAddr":
		subOid, indexOids := splitOid(indexOids, 4)
		parts := make([]string, 4)
		for i, o := range subOid {
			parts[i] = strconv.Itoa(o)
		}
		return strings.Join(parts, "."), subOid, indexOids
	case "InetAddressType":
		subOid, indexOids := splitOid(indexOids, 1)
		switch subOid[0] {
		case 0:
			return "unknown", subOid, indexOids
		case 1:
			return "ipv4", subOid, indexOids
		case 2:
			return "ipv6", subOid, indexOids
		case 3:
			return "ipv4z", subOid, indexOids
		case 4:
			return "ipv6z", subOid, indexOids
		case 16:
			return "dns", subOid, indexOids
		default:
			return strconv.Itoa(subOid[0]), subOid, indexOids
		}
	default:
		log.Fatalf("Unknown index type %s", typ)
		return "", nil, nil
	}
}

func indexesToLabels(indexOids []int, metric *config.Metric, oidToPdu map[string]gosnmp.SnmpPDU) map[string]string {
	labels := map[string]string{}
	labelOids := map[string][]int{}

	// Covert indexes to useful strings.
	for _, index := range metric.Indexes {
		str, subOid, remainingOids := indexOidsAsString(indexOids, index.Type, index.FixedSize)
		// The labelvalue is the text form of the index oids.
		labels[index.Labelname] = str
		// Save its oid in case we need it for lookups.
		labelOids[index.Labelname] = subOid
		// For the next iteration.
		indexOids = remainingOids
	}

	// Perform lookups.
	for _, lookup := range metric.Lookups {
		oid := lookup.Oid
		for _, label := range lookup.Labels {
			for _, o := range labelOids[label] {
				oid = fmt.Sprintf("%s.%d", oid, o)
			}
		}
		if pdu, ok := oidToPdu[oid]; ok {
			labels[lookup.Labelname] = pduValueAsString(&pdu, lookup.Type)
		} else {
			labels[lookup.Labelname] = ""
		}
	}

	return labels
}
