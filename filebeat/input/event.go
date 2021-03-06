package input

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
)

// FileEvent is sent to the output and must contain all relevant information
type FileEvent struct {
	common.EventMetadata
	ReadTime     time.Time
	Source       string
	InputType    string
	DocumentType string
	Offset       int64
	Bytes        int
	Text         *string
	Fileinfo     os.FileInfo
	JSONFields   common.MapStr
	JSONConfig   *JSONConfig
	FileState    FileState
}

type JSONConfig struct {
	MessageKey    string `config:"message_key"`
	KeysUnderRoot bool   `config:"keys_under_root"`
	OverwriteKeys bool   `config:"overwrite_keys"`
	AddErrorKey   bool   `config:"add_error_key"`
}

type MultilineConfig struct {
	Negate   bool           `config:"negate"`
	Match    string         `config:"match"       validate:"required"`
	MaxLines *int           `config:"max_lines"`
	Pattern  *regexp.Regexp `config:"pattern"`
	Timeout  *time.Duration `config:"timeout"     validate:"positive"`
}

func (c *MultilineConfig) Validate() error {
	if c.Match != "after" && c.Match != "before" {
		return fmt.Errorf("unknown matcher type: %s", c.Match)
	}
	return nil
}

// mergeJSONFields writes the JSON fields in the event map,
// respecting the KeysUnderRoot and OverwriteKeys configuration options.
// If MessageKey is defined, the Text value from the event always
// takes precedence.
func mergeJSONFields(f *FileEvent, event common.MapStr) {

	// The message key might have been modified by multiline
	if len(f.JSONConfig.MessageKey) > 0 && f.Text != nil {
		f.JSONFields[f.JSONConfig.MessageKey] = *f.Text
	}

	if f.JSONConfig.KeysUnderRoot {
		for k, v := range f.JSONFields {
			if f.JSONConfig.OverwriteKeys {
				if k == "@timestamp" {
					vstr, ok := v.(string)
					if !ok {
						logp.Err("JSON: Won't overwrite @timestamp because value is not string")
						event[jsonErrorKey] = "@timestamp not overwritten (not string)"
						continue
					}

					// @timestamp must be of format RFC3339
					ts, err := time.Parse(time.RFC3339, vstr)
					if err != nil {
						logp.Err("JSON: Won't overwrite @timestamp because of parsing error: %v", err)
						event[jsonErrorKey] = fmt.Sprintf("@timestamp not overwritten (parse error on %s)", vstr)
						continue
					}
					event[k] = common.Time(ts)
				} else if k == "type" {
					vstr, ok := v.(string)
					if !ok {
						logp.Err("JSON: Won't overwrite type because value is not string")
						event[jsonErrorKey] = "type not overwritten (not string)"
						continue
					}
					if len(vstr) == 0 || vstr[0] == '_' {
						logp.Err("JSON: Won't overwrite type because value is empty or starts with an underscore")
						event[jsonErrorKey] = fmt.Sprintf("type not overwritten (invalid value [%s])", vstr)
						continue
					}
					event[k] = vstr
				} else {
					event[k] = v
				}
			} else if _, exists := event[k]; !exists {
				event[k] = v
			}
		}
	} else {
		event["json"] = f.JSONFields
	}
}

func (f *FileEvent) ToMapStr() common.MapStr {
	event := common.MapStr{
		common.EventMetadataKey: f.EventMetadata,
		"@timestamp":            common.Time(f.ReadTime),
		"source":                f.Source,
		"offset":                f.Offset, // Offset here is the offset before the starting char.
		"type":                  f.DocumentType,
		"input_type":            f.InputType,
	}

	if f.JSONConfig != nil && len(f.JSONFields) > 0 {
		mergeJSONFields(f, event)
	} else {
		event["message"] = f.Text
	}

	return event
}
