package generator

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	reMessageID        = regexp.MustCompile(`(?s)(<MessageID>)(.*?)(</MessageID>)`)
	reDocumentID       = regexp.MustCompile(`(?s)(<DocumentID>)(.*?)(</DocumentID>)`)
	reDeliveryNoteID   = regexp.MustCompile(`(?s)(<DeliveryNoteID>)(.*?)(</DeliveryNoteID>)`)
	reMsgSenderID      = regexp.MustCompile(`(?s)(<MsgSenderID>)(.*?)(</MsgSenderID>)`)
	reMsgDateTime      = regexp.MustCompile(`(?s)(<MsgDateTime>)(.*?)(</MsgDateTime>)`)
	reCreationDateTime = regexp.MustCompile(`(?s)(<CreationDateTime>)(.*?)(</CreationDateTime>)`)
	reDeliveryNoteDate = regexp.MustCompile(`(?s)(<DeliveryNoteDate>)(.*?)(</DeliveryNoteDate>)`)
	reShipperGLN       = regexp.MustCompile(`(?s)(<Shipper>[\s\S]*?<GLN>)(.*?)(</GLN>)`)
	reFreightPayerGLN  = regexp.MustCompile(`(?s)(<FreightPayer>[\s\S]*?<GLN>)(.*?)(</GLN>)`)
	reSuffixDigits     = regexp.MustCompile(`^(.*?)(\d+)$`)
)

type Config struct {
	TemplateXML         string
	BaseMessageID       string
	BaseDocumentID      string
	BaseDeliveryNoteID  string
	OriginalGLN         string
	GLN                 string
	Suffix              string
	UpdateTimestamps    bool
}

type Generator struct {
	cfg Config
}

func NewGenerator(cfg Config) (*Generator, error) {
	if cfg.TemplateXML == "" {
		return nil, fmt.Errorf("template XML cannot be empty")
	}

	// Auto-extract defaults from template if not provided
	if cfg.BaseMessageID == "" {
		m := reMessageID.FindStringSubmatch(cfg.TemplateXML)
		if len(m) >= 3 {
			cfg.BaseMessageID = strings.TrimSpace(m[2])
		}
	}

	if cfg.BaseDocumentID == "" {
		m := reDocumentID.FindStringSubmatch(cfg.TemplateXML)
		if len(m) >= 3 {
			cfg.BaseDocumentID = strings.TrimSpace(m[2])
		}
	}

	if cfg.BaseDeliveryNoteID == "" {
		m := reDeliveryNoteID.FindStringSubmatch(cfg.TemplateXML)
		if len(m) >= 3 {
			cfg.BaseDeliveryNoteID = strings.TrimSpace(m[2])
		}
	}

	if cfg.OriginalGLN == "" {
		m := reMsgSenderID.FindStringSubmatch(cfg.TemplateXML)
		if len(m) >= 3 {
			cfg.OriginalGLN = strings.TrimSpace(m[2])
		}
	}

	if cfg.GLN == "" {
		cfg.GLN = cfg.OriginalGLN
	}

	return &Generator{cfg: cfg}, nil
}

// FormatSuffix formats the suffix for document at 0-based index.
// Examples:
// - suffix="001", index=0 -> "001"
// - suffix="001", index=1 -> "002"
// - suffix="batch_001", index=1 -> "batch_002"
// - suffix="test", index=0 -> "test001"
// - suffix="", index=0 -> "001"
func FormatSuffix(suffix string, index int) string {
	suffix = strings.TrimSpace(suffix)
	if suffix == "" {
		return fmt.Sprintf("%03d", index+1)
	}

	match := reSuffixDigits.FindStringSubmatch(suffix)
	if len(match) == 3 {
		prefix := match[1]
		digits := match[2]
		num, err := strconv.Atoi(digits)
		if err == nil {
			currentNum := num + index
			formatStr := fmt.Sprintf("%%0%dd", len(digits))
			return prefix + fmt.Sprintf(formatStr, currentNum)
		}
	}

	// Non-numeric suffix, append 3-digit index
	return fmt.Sprintf("%s%03d", suffix, index+1)
}

type GeneratedDoc struct {
	Index          int    `json:"index"`
	MessageID      string `json:"messageID"`
	DocumentID     string `json:"documentID"`
	DeliveryNoteID string `json:"deliveryNoteID"`
	GLN            string `json:"gln"`
	XML            string `json:"xml"`
}

func (g *Generator) Generate(index int) (*GeneratedDoc, error) {
	suffixVal := FormatSuffix(g.cfg.Suffix, index)

	msgID := g.cfg.BaseMessageID + suffixVal
	docID := g.cfg.BaseDocumentID + suffixVal

	// DeliveryNoteID handling:
	// Original example: 001-4810413900004-260616000002408
	// If GLN changed, replace old GLN in DeliveryNoteID with new GLN.
	baseDN := g.cfg.BaseDeliveryNoteID
	if g.cfg.OriginalGLN != "" && g.cfg.GLN != "" && g.cfg.OriginalGLN != g.cfg.GLN {
		baseDN = strings.Replace(baseDN, g.cfg.OriginalGLN, g.cfg.GLN, 1)
	}
	// Append suffix to make DeliveryNoteID unique
	dnID := baseDN + suffixVal

	res := g.cfg.TemplateXML

	// Replace MessageID
	res = reMessageID.ReplaceAllString(res, "${1}"+msgID+"${3}")

	// Replace DocumentID
	res = reDocumentID.ReplaceAllString(res, "${1}"+docID+"${3}")

	// Replace DeliveryNoteID
	res = reDeliveryNoteID.ReplaceAllString(res, "${1}"+dnID+"${3}")

	// Replace GLN fields if specified
	if g.cfg.GLN != "" {
		res = reMsgSenderID.ReplaceAllString(res, "${1}"+g.cfg.GLN+"${3}")
		res = reShipperGLN.ReplaceAllString(res, "${1}"+g.cfg.GLN+"${3}")
		res = reFreightPayerGLN.ReplaceAllString(res, "${1}"+g.cfg.GLN+"${3}")
	}

	// Timestamps if requested
	if g.cfg.UpdateTimestamps {
		now := time.Now()
		dtStr := now.Format("20060102150405")
		dateStr := now.Format("20060102")

		res = reMsgDateTime.ReplaceAllString(res, "${1}"+dtStr+"${3}")
		res = reCreationDateTime.ReplaceAllString(res, "${1}"+dtStr+"${3}")
		res = reDeliveryNoteDate.ReplaceAllString(res, "${1}"+dateStr+"${3}")
	}

	return &GeneratedDoc{
		Index:          index,
		MessageID:      msgID,
		DocumentID:     docID,
		DeliveryNoteID: dnID,
		GLN:            g.cfg.GLN,
		XML:            res,
	}, nil
}
