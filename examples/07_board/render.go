package main

import (
	"fmt"
	"html"
	"math"
	"strings"
)

// norm returns the bounding box of an element with positive width/height.
func norm(e *Element) (x, y, w, h float64) {
	switch e.Kind {
	case "pen":
		if len(e.Points) == 0 {
			return e.X, e.Y, 0, 0
		}
		minX, minY := e.Points[0].X, e.Points[0].Y
		maxX, maxY := minX, minY
		for _, p := range e.Points {
			minX, minY = math.Min(minX, p.X), math.Min(minY, p.Y)
			maxX, maxY = math.Max(maxX, p.X), math.Max(maxY, p.Y)
		}
		return minX, minY, maxX - minX, maxY - minY
	case "text":
		w := float64(len([]rune(e.Text))) * 12
		return e.X, e.Y - 22, w, 28
	}
	x, y, w, h = e.X, e.Y, e.W, e.H
	if w < 0 {
		x, w = x+w, -w
	}
	if h < 0 {
		y, h = y+h, -h
	}
	return
}

func fillAttr(e *Element) string {
	if e.Fill == "" || e.Fill == "transparent" {
		return `fill="none"`
	}
	return fmt.Sprintf(`fill="%s" fill-opacity="0.45"`, e.Fill)
}

func svgEl(e *Element) string {
	common := fmt.Sprintf(`stroke="%s" stroke-width="%.0f" stroke-linecap="round" stroke-linejoin="round"`, e.Stroke, e.StrokeW)
	switch e.Kind {
	case "pen":
		var pts strings.Builder
		for _, p := range e.Points {
			fmt.Fprintf(&pts, "%.1f,%.1f ", p.X, p.Y)
		}
		return fmt.Sprintf(`<polyline points="%s" fill="none" %s/>`, pts.String(), common)
	case "rect":
		x, y, w, h := norm(e)
		return fmt.Sprintf(`<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="8" %s %s/>`, x, y, w, h, fillAttr(e), common)
	case "ellipse":
		x, y, w, h := norm(e)
		return fmt.Sprintf(`<ellipse cx="%.1f" cy="%.1f" rx="%.1f" ry="%.1f" %s %s/>`, x+w/2, y+h/2, w/2, h/2, fillAttr(e), common)
	case "diamond":
		x, y, w, h := norm(e)
		return fmt.Sprintf(`<polygon points="%.1f,%.1f %.1f,%.1f %.1f,%.1f %.1f,%.1f" %s %s/>`,
			x+w/2, y, x+w, y+h/2, x+w/2, y+h, x, y+h/2, fillAttr(e), common)
	case "line":
		return fmt.Sprintf(`<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" %s/>`, e.X, e.Y, e.X+e.W, e.Y+e.H, common)
	case "arrow":
		x2, y2 := e.X+e.W, e.Y+e.H
		ang := math.Atan2(e.H, e.W)
		l := 14.0
		ax1 := x2 - l*math.Cos(ang-0.45)
		ay1 := y2 - l*math.Sin(ang-0.45)
		ax2 := x2 - l*math.Cos(ang+0.45)
		ay2 := y2 - l*math.Sin(ang+0.45)
		return fmt.Sprintf(`<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" %s/><polygon points="%.1f,%.1f %.1f,%.1f %.1f,%.1f" fill="%s"/>`,
			e.X, e.Y, x2, y2, common, x2, y2, ax1, ay1, ax2, ay2, e.Stroke)
	case "text":
		return fmt.Sprintf(`<text x="%.1f" y="%.1f" font-size="22" font-family="'Segoe Print','Comic Sans MS',cursive" fill="%s">%s</text>`,
			e.X, e.Y, e.Stroke, html.EscapeString(e.Text))
	}
	return ""
}

func renderElsLocked(b *Board) string {
	var sb strings.Builder
	for _, e := range b.Els {
		sb.WriteString(svgEl(e))
	}
	return sb.String()
}

// renderOverlayLocked draws this session's selection box plus the live
// cursors of every other user on the same board.
func renderOverlayLocked(s *session) string {
	var sb strings.Builder
	if s.selected != 0 {
		if el, _ := getBoard(s.board).find(s.selected); el != nil {
			x, y, w, h := norm(el)
			sb.WriteString(fmt.Sprintf(`<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="none" stroke="#4f46e5" stroke-width="1.5" stroke-dasharray="6 4"/>`,
				x-6, y-6, w+12, h+12))
		}
	}
	for _, o := range sessions {
		if o.lid == s.lid || o.board != s.board || !o.hasCursor {
			continue
		}
		sb.WriteString(fmt.Sprintf(`<g transform="translate(%.1f,%.1f)"><path d="M0 0 L14 11 L6 12 L9 19 Z" fill="%s"/><text x="14" y="24" font-size="12" fill="%s">%s</text></g>`,
			o.curX, o.curY, o.color, o.color, html.EscapeString(o.name)))
	}
	return sb.String()
}

// hitTestLocked returns the topmost element whose bounding box contains the
// point, with a small tolerance so thin lines stay clickable.
func hitTestLocked(b *Board, x, y float64) *Element {
	const pad = 6
	for i := len(b.Els) - 1; i >= 0; i-- {
		e := b.Els[i]
		ex, ey, ew, eh := norm(e)
		if x >= ex-pad && x <= ex+ew+pad && y >= ey-pad && y <= ey+eh+pad {
			return e
		}
	}
	return nil
}
