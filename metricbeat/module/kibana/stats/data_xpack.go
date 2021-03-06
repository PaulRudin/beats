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

package stats

import (
	"encoding/json"
	"time"

	"github.com/elastic/beats/libbeat/common"
	s "github.com/elastic/beats/libbeat/common/schema"
	c "github.com/elastic/beats/libbeat/common/schema/mapstriface"
	"github.com/elastic/beats/metricbeat/helper/elastic"
	"github.com/elastic/beats/metricbeat/mb"
)

var (
	schemaXPackMonitoring = s.Schema{
		"concurrent_connections": c.Int("concurrent_connections"),
		"os": c.Dict("os", s.Schema{
			"load": c.Dict("load", s.Schema{
				"1m":  c.Float("1m"),
				"5m":  c.Float("5m"),
				"15m": c.Float("15m"),
			}),
			"memory": c.Dict("memory", s.Schema{
				"total_in_bytes": c.Int("total_bytes"),
				"free_in_bytes":  c.Int("free_bytes"),
				"used_in_bytes":  c.Int("used_bytes"),
			}),
			"uptime_in_millis": c.Int("uptime_ms"),
		}),
		"process": c.Dict("process", s.Schema{
			"event_loop_delay": c.Float("event_loop_delay"),
			"memory": c.Dict("memory", s.Schema{
				"heap": c.Dict("heap", s.Schema{
					"total_in_bytes": c.Int("total_bytes"),
					"used_in_bytes":  c.Int("used_bytes"),
					"size_limit":     c.Int("size_limit"),
				}),
			}),
			"uptime_in_millis": c.Int("uptime_ms"),
		}),
		"requests": RequestsDict,
		"response_times": c.Dict("response_times", s.Schema{
			"average": c.Int("avg_ms", s.Optional),
			"max":     c.Int("max_ms", s.Optional),
		}, c.DictOptional),
		"kibana": c.Dict("kibana", s.Schema{
			"uuid":              c.Str("uuid"),
			"name":              c.Str("name"),
			"index":             c.Str("index"),
			"host":              c.Str("host"),
			"transport_address": c.Str("transport_address"),
			"version":           c.Str("version"),
			"snapshot":          c.Bool("snapshot"),
			"status":            c.Str("status"),
		}),
		"usage": c.Dict("usage", s.Schema{
			"index": c.Str("kibana.index"),
			"index_pattern": c.Dict("kibana.index_pattern", s.Schema{
				"total": c.Int("total"),
			}),
			"search": c.Dict("kibana.search", s.Schema{
				"total": c.Int("total"),
			}),
			"visualization": c.Dict("kibana.visualization", s.Schema{
				"total": c.Int("total"),
			}),
			"dashboard": c.Dict("kibana.dashboard", s.Schema{
				"total": c.Int("total"),
			}),
			"timelion_sheet": c.Dict("kibana.timelion_sheet", s.Schema{
				"total": c.Int("total"),
			}),
			"graph_workspace": c.Dict("kibana.graph_workspace", s.Schema{
				"total": c.Int("total"),
			}),
			"xpack": s.Object{
				"reporting": c.Dict("reporting", s.Schema{
					"available":     c.Bool("available"),
					"enabled":       c.Bool("enabled"),
					"browser_type":  c.Str("browser_type"),
					"_all":          c.Int("all"),
					"csv":           reportingCsvDict,
					"printable_pdf": reportingPrintablePdfDict,
					"status":        reportingStatusDict,
					"lastDay":       c.Dict("last_day", reportingPeriodSchema, c.DictOptional),
					"last7Days":     c.Dict("last_7_days", reportingPeriodSchema, c.DictOptional),
				}, c.DictOptional),
			},
		}),
	}

	reportingCsvDict = c.Dict("csv", s.Schema{
		"available": c.Bool("available"),
		"total":     c.Int("total"),
	}, c.DictOptional)

	reportingPrintablePdfDict = c.Dict("printable_pdf", s.Schema{
		"available": c.Bool("available"),
		"total":     c.Int("total"),
		"app": c.Dict("app", s.Schema{
			"visualization": c.Int("visualization"),
			"dashboard":     c.Int("dashboard"),
		}, c.DictOptional),
		"layout": c.Dict("layout", s.Schema{
			"print":           c.Int("print"),
			"preserve_layout": c.Int("preserve_layout"),
		}, c.DictOptional),
	}, c.DictOptional)

	reportingStatusDict = c.Dict("status", s.Schema{
		"completed":  c.Int("completed", s.Optional),
		"failed":     c.Int("failed", s.Optional),
		"processing": c.Int("processing", s.Optional),
		"pending":    c.Int("pending", s.Optional),
	}, c.DictOptional)

	reportingPeriodSchema = s.Schema{
		"_all":          c.Int("all"),
		"csv":           reportingCsvDict,
		"printable_pdf": reportingPrintablePdfDict,
		"status":        reportingStatusDict,
	}
)

func eventMappingXPack(r mb.ReporterV2, intervalMs int64, content []byte) error {
	var data map[string]interface{}
	err := json.Unmarshal(content, &data)
	if err != nil {
		r.Error(err)
		return err
	}

	kibanaStatsFields, err := schemaXPackMonitoring.Apply(data)
	if err != nil {
		r.Error(err)
		return err
	}

	process, ok := data["process"].(map[string]interface{})
	if !ok {
		return elastic.ReportErrorForMissingField("process", elastic.Kibana, r)
	}
	memory, ok := process["memory"].(map[string]interface{})
	if !ok {
		return elastic.ReportErrorForMissingField("process.memory", elastic.Kibana, r)
	}

	rss, ok := memory["resident_set_size_bytes"].(float64)
	if !ok {
		return elastic.ReportErrorForMissingField("process.memory.resident_set_size_bytes", elastic.Kibana, r)
	}
	kibanaStatsFields.Put("process.memory.resident_set_size_in_bytes", int64(rss))

	timestamp := time.Now()
	kibanaStatsFields.Put("timestamp", timestamp)

	var event mb.Event
	event.RootFields = common.MapStr{
		"cluster_uuid": data["cluster_uuid"].(string),
		"timestamp":    timestamp,
		"interval_ms":  intervalMs,
		"type":         "kibana_stats",
		"kibana_stats": kibanaStatsFields,
	}

	event.Index = elastic.MakeXPackMonitoringIndexName(elastic.Kibana)
	r.Event(event)

	return nil
}
