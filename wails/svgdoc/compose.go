package svgdoc

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"image/png"
	"math"
	"strconv"
	"strings"

	"snapvector/annotation"
)

const fontFamily = `-apple-system,BlinkMacSystemFont,"Noto Sans CJK TC","PingFang TC","Microsoft JhengHei",sans-serif`

const (
	baselineArrowViewWidth  = 240.0
	baselineArrowViewHeight = 120.0
	baselineArrowTailX      = 6.0
	baselineArrowTailY      = 60.0
	baselineArrowTipX       = 228.0
	baselineArrowTipY       = 60.0

	baselineRectViewWidth  = 220.0
	baselineRectViewHeight = 140.0
	baselineRectInsetX     = 14.0
	baselineRectInsetY     = 14.0
	baselineRectWidth      = 192.0
	baselineRectHeight     = 112.0
	baselineRectRadius     = 18.0

	baselineTextViewWidth   = 260.0
	baselineTextViewHeight  = 92.0
	baselineTextInset       = 8.0
	baselineTextRectWidth   = 244.0
	baselineTextRectHeight  = 76.0
	baselineTextRadius      = 18.0
	baselineTextStartX      = 26.0
	baselineTextBaselineY   = 56.0
	baselineTextFontSize    = 30.0
	baselineBlurOutlineW    = 4.0
	baselineBlurDashStrokeW = 6.0
)

type renderedAnnotation struct {
	def string
	use string
}

func Compose(basePNG []byte, canvasWidth, canvasHeight int, annotations []annotation.Annotation) (string, error) {
	if canvasWidth <= 0 || canvasHeight <= 0 {
		cfg, err := png.DecodeConfig(bytes.NewReader(basePNG))
		if err != nil {
			return "", fmt.Errorf("decode base PNG: %w", err)
		}
		canvasWidth = cfg.Width
		canvasHeight = cfg.Height
	}

	baseDataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(basePNG)
	rendered := make([]renderedAnnotation, 0, len(annotations))
	for idx, ann := range annotations {
		item, err := renderAnnotation(idx, ann, baseDataURL, canvasWidth, canvasHeight)
		if err != nil {
			return "", err
		}
		rendered = append(rendered, item)
	}

	var defs strings.Builder
	var uses strings.Builder
	for _, item := range rendered {
		defs.WriteString(item.def)
		uses.WriteString(item.use)
	}

	var svg strings.Builder
	fmt.Fprintf(&svg, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, canvasWidth, canvasHeight, canvasWidth, canvasHeight)
	svg.WriteString("<title>SnapVector injected composition</title>")
	svg.WriteString("<defs>")
	svg.WriteString(`<style>text{font-family:` + fontFamily + `;} .sv-text{font-weight:800;} .sv-vector{vector-effect:non-scaling-stroke;stroke-linecap:round;stroke-linejoin:round;} .sv-red-fill{fill:#E53935;} .sv-white-fill{fill:#FFFFFF;}</style>`)
	svg.WriteString(defs.String())
	svg.WriteString("</defs>")
	fmt.Fprintf(&svg, `<image x="0" y="0" width="%d" height="%d" href="%s"/>`, canvasWidth, canvasHeight, quoteAttr(baseDataURL))
	svg.WriteString(`<g id="annotations">`)
	svg.WriteString(uses.String())
	svg.WriteString(`</g></svg>`)

	return svg.String(), nil
}

func renderAnnotation(idx int, ann annotation.Annotation, baseDataURL string, canvasWidth, canvasHeight int) (renderedAnnotation, error) {
	switch ann.Type {
	case annotation.TypeArrow:
		return renderArrow(idx, ann), nil
	case annotation.TypeRectangle:
		return renderRectangle(idx, ann), nil
	case annotation.TypeEllipse:
		return renderEllipse(idx, ann), nil
	case annotation.TypeText:
		return renderText(idx, ann), nil
	case annotation.TypeBlur:
		return renderBlur(idx, ann, baseDataURL, canvasWidth, canvasHeight), nil
	default:
		return renderedAnnotation{}, fmt.Errorf("unsupported annotation type %q", ann.Type)
	}
}

func renderArrow(idx int, ann annotation.Annotation) renderedAnnotation {
	dx := ann.X2 - ann.X1
	dy := ann.Y2 - ann.Y1
	length := math.Hypot(dx, dy)
	ux := dx / length
	uy := dy / length
	sx := length / (baselineArrowTipX - baselineArrowTailX)
	sy := ann.StrokeWidth / annotation.DefaultStrokeWidth
	if sy <= 0 {
		sy = 1
	}
	a := ux * sx
	b := uy * sx
	c := -uy * sy
	d := ux * sy
	e := ann.X1 - a*baselineArrowTailX - c*baselineArrowTailY
	f := ann.Y1 - b*baselineArrowTailX - d*baselineArrowTailY

	baselinePoints := [][2]float64{
		{6, 60},
		{132, 48},
		{132, 24},
		{228, 60},
		{132, 96},
		{132, 72},
	}

	points := make([][2]float64, 0, len(baselinePoints))
	for _, point := range baselinePoints {
		points = append(points, [2]float64{
			a*point[0] + c*point[1] + e,
			b*point[0] + d*point[1] + f,
		})
	}

	minX, minY, maxX, maxY := polygonBounds(points)
	padding := ann.StrokeWidth
	symbolID := symbolID(idx, ann.ID)

	var polygon strings.Builder
	for i, point := range points {
		if i > 0 {
			polygon.WriteByte(' ')
		}
		polygon.WriteString(formatFloat(point[0] - minX + padding))
		polygon.WriteByte(',')
		polygon.WriteString(formatFloat(point[1] - minY + padding))
	}

	width := maxX - minX + padding*2
	height := maxY - minY + padding*2

	outlineSW := 8 * (ann.StrokeWidth / annotation.DefaultStrokeWidth)

	def := fmt.Sprintf(
		`<symbol id="%s" viewBox="0 0 %s %s">`+
			`<polygon points="%s" fill="%s" stroke="%s" stroke-width="%s" stroke-linejoin="round"/>`+
			`<polygon points="%s" fill="%s" stroke="none"/>`+
			`</symbol>`,
		symbolID,
		formatFloat(width),
		formatFloat(height),
		polygon.String(),
		quoteAttr(ann.OutlineColor),
		quoteAttr(ann.OutlineColor),
		formatFloat(outlineSW),
		polygon.String(),
		quoteAttr(ann.StrokeColor),
	)
	use := fmt.Sprintf(`<use href="#%s" x="%s" y="%s" width="%s" height="%s"/>`,
		symbolID,
		formatFloat(minX-padding),
		formatFloat(minY-padding),
		formatFloat(width),
		formatFloat(height),
	)

	return renderedAnnotation{def: def, use: use}
}

func renderRectangle(idx int, ann annotation.Annotation) renderedAnnotation {
	outlineWidth := ann.StrokeWidth + 6
	insetX := ann.Width * (baselineRectInsetX / baselineRectViewWidth)
	insetY := ann.Height * (baselineRectInsetY / baselineRectViewHeight)
	rectWidth := ann.Width * (baselineRectWidth / baselineRectViewWidth)
	rectHeight := ann.Height * (baselineRectHeight / baselineRectViewHeight)
	radius := math.Min(
		math.Min(ann.Width*(baselineRectRadius/baselineRectViewWidth), ann.Height*(baselineRectRadius/baselineRectViewHeight)),
		math.Min(ann.Width, ann.Height)/2,
	)
	symbolID := symbolID(idx, ann.ID)

	def := fmt.Sprintf(
		`<symbol id="%s" viewBox="0 0 %s %s"><rect x="%s" y="%s" width="%s" height="%s" rx="%s" fill="none" stroke="%s" stroke-width="%s" class="sv-vector"/><rect x="%s" y="%s" width="%s" height="%s" rx="%s" fill="none" stroke="%s" stroke-width="%s" class="sv-vector"/></symbol>`,
		symbolID,
		formatFloat(ann.Width),
		formatFloat(ann.Height),
		formatFloat(insetX),
		formatFloat(insetY),
		formatFloat(rectWidth),
		formatFloat(rectHeight),
		formatFloat(radius),
		quoteAttr(ann.OutlineColor),
		formatFloat(outlineWidth),
		formatFloat(insetX),
		formatFloat(insetY),
		formatFloat(rectWidth),
		formatFloat(rectHeight),
		formatFloat(radius),
		quoteAttr(ann.StrokeColor),
		formatFloat(ann.StrokeWidth),
	)
	use := fmt.Sprintf(`<use href="#%s" x="%s" y="%s" width="%s" height="%s"/>`,
		symbolID,
		formatFloat(ann.X),
		formatFloat(ann.Y),
		formatFloat(ann.Width),
		formatFloat(ann.Height),
	)

	return renderedAnnotation{def: def, use: use}
}

func renderEllipse(idx int, ann annotation.Annotation) renderedAnnotation {
	outlineWidth := ann.StrokeWidth + 6
	symbolID := symbolID(idx, ann.ID)
	cx := ann.Width * (110.0 / baselineRectViewWidth)
	cy := ann.Height * (70.0 / baselineRectViewHeight)
	rx := ann.Width * (92.0 / baselineRectViewWidth)
	ry := ann.Height * (52.0 / baselineRectViewHeight)

	def := fmt.Sprintf(
		`<symbol id="%s" viewBox="0 0 %s %s"><ellipse cx="%s" cy="%s" rx="%s" ry="%s" fill="none" stroke="%s" stroke-width="%s" class="sv-vector"/><ellipse cx="%s" cy="%s" rx="%s" ry="%s" fill="none" stroke="%s" stroke-width="%s" class="sv-vector"/></symbol>`,
		symbolID,
		formatFloat(ann.Width),
		formatFloat(ann.Height),
		formatFloat(cx),
		formatFloat(cy),
		formatFloat(rx),
		formatFloat(ry),
		quoteAttr(ann.OutlineColor),
		formatFloat(outlineWidth),
		formatFloat(cx),
		formatFloat(cy),
		formatFloat(rx),
		formatFloat(ry),
		quoteAttr(ann.StrokeColor),
		formatFloat(ann.StrokeWidth),
	)
	use := fmt.Sprintf(`<use href="#%s" x="%s" y="%s" width="%s" height="%s"/>`,
		symbolID,
		formatFloat(ann.X),
		formatFloat(ann.Y),
		formatFloat(ann.Width),
		formatFloat(ann.Height),
	)

	return renderedAnnotation{def: def, use: use}
}

func renderText(idx int, ann annotation.Annotation) renderedAnnotation {
	lines := annotation.WrapText(ann.Text, ann.MaxWidth, ann.FontSize)
	longest := 0
	for _, line := range lines {
		if len([]rune(line)) > longest {
			longest = len([]rune(line))
		}
	}
	if longest == 0 {
		longest = 1
	}

	contentWidth := math.Max(float64(longest)*ann.FontSize*0.92, ann.FontSize*2)
	if ann.MaxWidth > 0 && contentWidth > ann.MaxWidth {
		contentWidth = ann.MaxWidth
	}

	scale := ann.FontSize / baselineTextFontSize
	margin := baselineTextInset * scale
	paddingX := (baselineTextStartX - baselineTextInset) * scale
	paddingTop := (baselineTextBaselineY - baselineTextFontSize - baselineTextInset) * scale
	lineHeight := ann.FontSize * 1.25
	boxWidth := contentWidth + paddingX*2 + margin*2
	boxHeight := float64(len(lines))*lineHeight + paddingTop + margin + ann.FontSize*0.35
	if len(lines) == 0 {
		boxHeight = ann.FontSize + paddingTop + margin*2
	}
	symbolID := symbolID(idx, ann.ID)
	rectWidth := math.Max(boxWidth-margin*2, baselineTextRectWidth*scale)
	rectHeight := math.Max(boxHeight-margin*2, baselineTextRectHeight*scale)
	radius := baselineTextRadius * scale

	var box strings.Builder
	switch ann.Variant {
	case "outline":
		fmt.Fprintf(&box, `<rect x="%s" y="%s" width="%s" height="%s" rx="%s" fill="#FFFFFF" stroke="#FFFFFF" stroke-width="%s"/>`, formatFloat(margin), formatFloat(margin), formatFloat(rectWidth), formatFloat(rectHeight), formatFloat(radius), formatFloat(3*scale))
		fmt.Fprintf(&box, `<rect x="%s" y="%s" width="%s" height="%s" rx="%s" fill="none" stroke="%s" stroke-width="%s"/>`, formatFloat(margin), formatFloat(margin), formatFloat(rectWidth), formatFloat(rectHeight), formatFloat(radius), quoteAttr(ann.StrokeColor), formatFloat(8*scale))
	default:
		fmt.Fprintf(&box, `<rect x="%s" y="%s" width="%s" height="%s" rx="%s" fill="%s" stroke="%s" stroke-width="%s" paint-order="stroke fill"/>`, formatFloat(margin), formatFloat(margin), formatFloat(rectWidth), formatFloat(rectHeight), formatFloat(radius), quoteAttr(ann.StrokeColor), quoteAttr(ann.OutlineColor), formatFloat(6*scale))
	}

	var textBuilder strings.Builder
	fill := "#FFFFFF"
	if ann.Variant == "outline" {
		fill = ann.StrokeColor
	}
	fmt.Fprintf(&textBuilder, `<text class="sv-text" x="%s" y="%s" font-size="%s" fill="%s">`,
		formatFloat(margin+paddingX),
		formatFloat(margin+paddingTop+ann.FontSize),
		formatFloat(ann.FontSize),
		quoteAttr(fill),
	)
	for lineIndex, line := range lines {
		dy := "0"
		if lineIndex > 0 {
			dy = formatFloat(lineHeight)
		}
		fmt.Fprintf(&textBuilder, `<tspan x="%s" dy="%s">%s</tspan>`,
			formatFloat(margin+paddingX),
			dy,
			escapeText(line),
		)
	}
	textBuilder.WriteString(`</text>`)

	def := fmt.Sprintf(`<symbol id="%s" viewBox="0 0 %s %s">%s%s</symbol>`,
		symbolID,
		formatFloat(boxWidth),
		formatFloat(boxHeight),
		box.String(),
		textBuilder.String(),
	)
	use := fmt.Sprintf(`<use href="#%s" x="%s" y="%s" width="%s" height="%s"/>`,
		symbolID,
		formatFloat(ann.X),
		formatFloat(ann.Y),
		formatFloat(boxWidth),
		formatFloat(boxHeight),
	)

	return renderedAnnotation{def: def, use: use}
}

func renderBlur(idx int, ann annotation.Annotation, baseDataURL string, canvasWidth, canvasHeight int) renderedAnnotation {
	clipID := symbolID(idx, ann.ID) + "-clip"
	filterID := symbolID(idx, ann.ID) + "-filter"
	symbolID := symbolID(idx, ann.ID)
	sigma := ann.BlurRadius / 3
	if sigma < 0.1 {
		sigma = 0.1
	}
	insetX := ann.Width * (baselineRectInsetX / baselineRectViewWidth)
	insetY := ann.Height * (baselineRectInsetY / baselineRectViewHeight)
	contentWidth := ann.Width * (baselineRectWidth / baselineRectViewWidth)
	contentHeight := ann.Height * (baselineRectHeight / baselineRectViewHeight)
	cornerRadius := math.Min(
		math.Min(ann.CornerRadius, contentWidth/2),
		contentHeight/2,
	)
	sx := ann.Width / baselineRectViewWidth
	sy := ann.Height / baselineRectViewHeight

	def := fmt.Sprintf(
		`<symbol id="%s" viewBox="0 0 %s %s"><rect x="%s" y="%s" width="%s" height="%s" rx="%s" fill="#FFFFFF" opacity="0.14"/><clipPath id="%s"><rect x="%s" y="%s" width="%s" height="%s" rx="%s"/></clipPath><filter id="%s" x="-25%%" y="-25%%" width="150%%" height="150%%"><feGaussianBlur stdDeviation="%s"/></filter><image x="%s" y="%s" width="%s" height="%s" href="%s" clip-path="url(#%s)" filter="url(#%s)"/><rect x="%s" y="%s" width="%s" height="%s" rx="%s" fill="none" stroke="%s" stroke-width="%s"/><rect x="%s" y="%s" width="%s" height="%s" rx="%s" fill="none" stroke="%s" stroke-width="%s" stroke-dasharray="10 6"/></symbol>`,
		symbolID,
		formatFloat(ann.Width),
		formatFloat(ann.Height),
		formatFloat(insetX),
		formatFloat(insetY),
		formatFloat(contentWidth),
		formatFloat(contentHeight),
		formatFloat(cornerRadius),
		clipID,
		formatFloat(insetX),
		formatFloat(insetY),
		formatFloat(contentWidth),
		formatFloat(contentHeight),
		formatFloat(cornerRadius),
		filterID,
		formatFloat(sigma),
		formatFloat(-ann.X/sx),
		formatFloat(-ann.Y/sy),
		formatFloat(float64(canvasWidth)/sx),
		formatFloat(float64(canvasHeight)/sy),
		quoteAttr(baseDataURL),
		clipID,
		filterID,
		formatFloat(insetX),
		formatFloat(insetY),
		formatFloat(contentWidth),
		formatFloat(contentHeight),
		formatFloat(cornerRadius),
		quoteAttr(ann.OutlineColor),
		formatFloat(baselineBlurOutlineW),
		formatFloat(insetX),
		formatFloat(insetY),
		formatFloat(contentWidth),
		formatFloat(contentHeight),
		formatFloat(cornerRadius),
		quoteAttr(ann.StrokeColor),
		formatFloat(baselineBlurDashStrokeW),
	)
	use := fmt.Sprintf(`<use href="#%s" x="%s" y="%s" width="%s" height="%s"/>`,
		symbolID,
		formatFloat(ann.X),
		formatFloat(ann.Y),
		formatFloat(ann.Width),
		formatFloat(ann.Height),
	)

	return renderedAnnotation{def: def, use: use}
}

func polygonBounds(points [][2]float64) (float64, float64, float64, float64) {
	minX := points[0][0]
	minY := points[0][1]
	maxX := points[0][0]
	maxY := points[0][1]

	for _, point := range points[1:] {
		if point[0] < minX {
			minX = point[0]
		}
		if point[1] < minY {
			minY = point[1]
		}
		if point[0] > maxX {
			maxX = point[0]
		}
		if point[1] > maxY {
			maxY = point[1]
		}
	}

	return minX, minY, maxX, maxY
}

func symbolID(idx int, id string) string {
	return fmt.Sprintf("ann-symbol-%d-%s", idx, sanitizeID(id))
}

func sanitizeID(id string) string {
	var builder strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r + ('a' - 'A'))
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteByte('-')
		}
	}
	if builder.Len() == 0 {
		return "annotation"
	}
	return builder.String()
}

func formatFloat(value float64) string {
	if math.Abs(value-math.Round(value)) < 1e-9 {
		value = math.Round(value)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func escapeText(text string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(text))
	return buf.String()
}

func quoteAttr(value string) string {
	return escapeText(value)
}
