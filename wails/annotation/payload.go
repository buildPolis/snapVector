package annotation

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	TypeArrow     = "arrow"
	TypeRectangle = "rectangle"
	TypeEllipse   = "ellipse"
	TypeText      = "text"
	TypeBlur      = "blur"

	DefaultStrokeColor  = "#E53935"
	DefaultOutlineColor = "#FFFFFF"
	DefaultStrokeWidth  = 10.0
	DefaultFontSize     = 30.0
	DefaultTextVariant  = "solid"
	DefaultBlurRadius   = 12.0
	DefaultCornerRadius = 18.0
)

var hexColorPattern = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

type Annotation struct {
	ID           string
	Type         string
	StrokeColor  string
	OutlineColor string
	StrokeWidth  float64

	X      float64
	Y      float64
	Width  float64
	Height float64

	X1 float64
	Y1 float64
	X2 float64
	Y2 float64

	Text     string
	Variant  string
	FontSize float64
	MaxWidth float64

	BlurRadius   float64
	CornerRadius float64
	Feather      float64
}

type rawAnnotation struct {
	ID           string   `json:"id"`
	Type         string   `json:"type"`
	StrokeColor  string   `json:"strokeColor"`
	OutlineColor string   `json:"outlineColor"`
	StrokeWidth  *float64 `json:"strokeWidth"`

	X      *float64 `json:"x"`
	Y      *float64 `json:"y"`
	Width  *float64 `json:"width"`
	Height *float64 `json:"height"`

	X1 *float64 `json:"x1"`
	Y1 *float64 `json:"y1"`
	X2 *float64 `json:"x2"`
	Y2 *float64 `json:"y2"`

	Text     *string  `json:"text"`
	Variant  string   `json:"variant"`
	FontSize *float64 `json:"fontSize"`
	MaxWidth *float64 `json:"maxWidth"`

	BlurRadius   *float64 `json:"blurRadius"`
	CornerRadius *float64 `json:"cornerRadius"`
	Feather      *float64 `json:"feather"`
}

func ParsePayload(payload string) ([]Annotation, error) {
	var raws []rawAnnotation

	dec := json.NewDecoder(strings.NewReader(payload))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raws); err != nil {
		return nil, fmt.Errorf("invalid inject payload: %w", err)
	}

	annotations := make([]Annotation, 0, len(raws))
	for idx, raw := range raws {
		annotation, err := normalize(idx, raw)
		if err != nil {
			return nil, fmt.Errorf("annotation %d: %w", idx, err)
		}
		annotations = append(annotations, annotation)
	}

	return annotations, nil
}

func normalize(idx int, raw rawAnnotation) (Annotation, error) {
	annotation := Annotation{
		ID:           raw.ID,
		Type:         raw.Type,
		StrokeColor:  normalizeColor(raw.StrokeColor, DefaultStrokeColor),
		OutlineColor: normalizeColor(raw.OutlineColor, DefaultOutlineColor),
		StrokeWidth:  defaultFloat(raw.StrokeWidth, DefaultStrokeWidth),
		Variant:      raw.Variant,
		FontSize:     defaultFloat(raw.FontSize, DefaultFontSize),
		MaxWidth:     optionalFloat(raw.MaxWidth),
		BlurRadius:   defaultFloat(raw.BlurRadius, DefaultBlurRadius),
		CornerRadius: defaultFloat(raw.CornerRadius, DefaultCornerRadius),
		Feather:      optionalFloat(raw.Feather),
	}

	if annotation.ID == "" {
		annotation.ID = fmt.Sprintf("ann-%d", idx+1)
	}
	if annotation.Type == "" {
		return Annotation{}, fmt.Errorf("missing type")
	}
	if !validColor(annotation.StrokeColor) {
		return Annotation{}, fmt.Errorf("invalid strokeColor %q", annotation.StrokeColor)
	}
	if !validColor(annotation.OutlineColor) {
		return Annotation{}, fmt.Errorf("invalid outlineColor %q", annotation.OutlineColor)
	}
	if annotation.StrokeWidth <= 0 {
		return Annotation{}, fmt.Errorf("strokeWidth must be greater than 0")
	}

	switch annotation.Type {
	case TypeArrow:
		if raw.X1 == nil || raw.Y1 == nil || raw.X2 == nil || raw.Y2 == nil {
			return Annotation{}, fmt.Errorf("arrow requires x1, y1, x2, y2")
		}
		annotation.X1 = *raw.X1
		annotation.Y1 = *raw.Y1
		annotation.X2 = *raw.X2
		annotation.Y2 = *raw.Y2
		if annotation.X1 == annotation.X2 && annotation.Y1 == annotation.Y2 {
			return Annotation{}, fmt.Errorf("arrow start and end cannot be identical")
		}
	case TypeRectangle, TypeEllipse, TypeBlur:
		if raw.X == nil || raw.Y == nil || raw.Width == nil || raw.Height == nil {
			return Annotation{}, fmt.Errorf("%s requires x, y, width, height", annotation.Type)
		}
		annotation.X = *raw.X
		annotation.Y = *raw.Y
		annotation.Width = *raw.Width
		annotation.Height = *raw.Height
		if annotation.Width <= 0 || annotation.Height <= 0 {
			return Annotation{}, fmt.Errorf("%s width and height must be greater than 0", annotation.Type)
		}
		if annotation.Type == TypeBlur {
			if annotation.CornerRadius < 0 {
				return Annotation{}, fmt.Errorf("blur cornerRadius must be >= 0")
			}
			if annotation.BlurRadius <= 0 {
				return Annotation{}, fmt.Errorf("blur blurRadius must be > 0")
			}
			if annotation.Feather <= 0 {
				annotation.Feather = annotation.BlurRadius
			}
		}
	case TypeText:
		if raw.X == nil || raw.Y == nil || raw.Text == nil {
			return Annotation{}, fmt.Errorf("text requires x, y, text")
		}
		annotation.X = *raw.X
		annotation.Y = *raw.Y
		annotation.Text = *raw.Text
		if strings.TrimSpace(annotation.Text) == "" {
			return Annotation{}, fmt.Errorf("text cannot be empty")
		}
		if annotation.FontSize <= 0 {
			return Annotation{}, fmt.Errorf("fontSize must be greater than 0")
		}
		if annotation.MaxWidth < 0 {
			return Annotation{}, fmt.Errorf("maxWidth must be >= 0")
		}
		if annotation.Variant == "" {
			annotation.Variant = DefaultTextVariant
		}
		if annotation.Variant != "solid" && annotation.Variant != "outline" {
			return Annotation{}, fmt.Errorf("text variant must be solid or outline")
		}
	default:
		return Annotation{}, fmt.Errorf("unsupported type %q", annotation.Type)
	}

	return annotation, nil
}

func WrapText(text string, maxWidth, fontSize float64) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if maxWidth <= 0 {
		return strings.Split(text, "\n")
	}

	maxChars := int(maxWidth / (fontSize * 0.9))
	if maxChars < 1 {
		maxChars = 1
	}

	var lines []string
	for _, block := range strings.Split(text, "\n") {
		if utf8.RuneCountInString(block) <= maxChars {
			lines = append(lines, block)
			continue
		}

		runes := []rune(block)
		for len(runes) > 0 {
			take := maxChars
			if take > len(runes) {
				take = len(runes)
			}
			lines = append(lines, string(runes[:take]))
			runes = runes[take:]
		}
	}

	return lines
}

func defaultFloat(value *float64, fallback float64) float64 {
	if value == nil {
		return fallback
	}
	return *value
}

func optionalFloat(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func normalizeColor(color string, fallback string) string {
	if color == "" {
		return fallback
	}
	return color
}

func validColor(color string) bool {
	return hexColorPattern.MatchString(color)
}
