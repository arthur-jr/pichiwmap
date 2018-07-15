package pichiwmap

import (
	"math"
	"time"

	"github.com/gowasm/gopherwasm/js"
)

type (
	OnLatChanged  func(lat float64)
	OnLonChanged  func(lon float64)
	OnZoomChanged func(zoom float64)
)

type MapEvents struct {
	OnLatChanged  OnLatChanged
	OnLonChanged  OnLonChanged
	OnZoomChanged OnZoomChanged
}

func (e MapEvents) wrapEmpty() MapEvents {
	if e.OnLatChanged == nil {
		e.OnLatChanged = func(float64) {}
	}
	if e.OnLonChanged == nil {
		e.OnLonChanged = func(float64) {}
	}
	if e.OnZoomChanged == nil {
		e.OnZoomChanged = func(float64) {}
	}
	return e
}

type TileRenderer interface {
	RenderTiles(tiles map[string]*Tile)
	Viewport() (width, height float64)
}

func New(urlEr URLer, divEl js.Value, events MapEvents) (*Map, error) {
	doc := js.Global().Get("document")
	body := doc.Get("body")

	viewport := doc.Call("createElement", "canvas")

	onResize := func(event js.Value) {
		width := divEl.Get("offsetWidth").Int()
		height := divEl.Get("offsetHeight").Int()
		viewport.Set("width", width)
		viewport.Set("height", height)
	}

	onResize(js.Null())

	divEl.Call("appendChild", viewport)

	body.Call("addEventListener", "gesturechange", js.NewEventCallback(js.PreventDefault, func(event js.Value) {}), false)
	body.Call("addEventListener", "gesturestart", js.NewEventCallback(js.PreventDefault, func(event js.Value) {}), false)

	m := &Map{
		events:   events.wrapEmpty(),
		lat:      49.8951,
		lon:      -97.1384,
		zoom:     15,
		zoomStep: 0.1,
		step:     0.001,
		urlEr:    urlEr,
		viewport: viewport,
		maxZoom:  18,
		minZoom:  0,
	}

	window := js.Global().Get("window")
	window.Call("addEventListener", "resize", js.NewEventCallback(0, func(event js.Value) {
		onResize(event)
		m.Update()
	}))

	m.tlat = m.Lat()
	m.tlon = m.Lon()

	m.viewport.Call("addEventListener", "mousedown", js.NewEventCallback(js.PreventDefault, m.onMouseDown), false)
	m.viewport.Call("addEventListener", "mouseup", js.NewEventCallback(js.PreventDefault, m.onMouseUp), false)
	m.viewport.Call("addEventListener", "mousemove", js.NewEventCallback(js.PreventDefault, m.onMouseMove), false)
	m.viewport.Call("addEventListener", "touchstart", js.NewEventCallback(js.PreventDefault, m.onMouseDown), false)
	m.viewport.Call("addEventListener", "touchend", js.NewEventCallback(js.PreventDefault, m.onMouseUp), false)
	m.viewport.Call("addEventListener", "touchmove", js.NewEventCallback(js.PreventDefault, m.onMouseMove), false)

	doc.Call("addEventListener", "keyup", js.NewEventCallback(js.PreventDefault, m.onKeyUp), false)
	doc.Call("addEventListener", "keydown", js.NewEventCallback(js.PreventDefault, m.onKeyDown), false)
	doc.Call("addEventListener", "wheel", js.NewEventCallback(js.PreventDefault, m.wheel), false)

	return m, nil
}

type Map struct {
	tileRenderers []TileRenderer

	events MapEvents

	doc      js.Value
	viewport js.Value

	urlEr         URLer
	zoom          float64
	zoomStep      float64
	lat           float64
	lon           float64
	tlat          float64
	tlon          float64
	step          float64
	mouseStartX   int
	mouseStartY   int
	mouseStartLat float64
	mouseStartLon float64
	mouseDown     bool
	arrowDown     bool
	maxZoom       float64
	minZoom       float64
}

func (m *Map) Zoom() float64 {
	return m.zoom
}

func (m *Map) setZoom(zoom float64) {
	if m.zoom == zoom {
		return
	}
	m.zoom = zoom
	m.events.OnZoomChanged(zoom)
}

func (m *Map) Lat() float64 {
	return m.lat
}

func (m *Map) setLat(lat float64) {
	if m.lat == lat {
		return
	}
	m.lat = lat
	m.events.OnLatChanged(lat)
}

func (m *Map) Lon() float64 {
	return m.lon
}

func (m *Map) setLon(lon float64) {
	if m.lon == lon {
		return
	}
	m.lon = lon
	m.events.OnLonChanged(lon)
}

func (m *Map) SetPosition(zoom, lat, lon float64) {
	if zoom < m.minZoom || zoom > m.maxZoom {
		return
	}
	if zoom == m.zoom && lat == m.lat && lon == m.lon {
		return
	}
	m.setZoom(zoom)
	m.setLat(lat)
	m.setLon(lon)
	m.Update()
}

func (m *Map) AddTileRenderers(tr ...TileRenderer) {
	m.tileRenderers = append(m.tileRenderers, tr...)
}

func (m *Map) Canvas() js.Value {
	return m.viewport
}

func (m *Map) anim() {
	if m.Lat() == m.tlat && m.Lon() == m.tlon {
		return
	}

	if m.tlat > m.Lat() {
		m.setLat(m.Lat() + m.step)
		if m.Lat() > m.tlat {
			m.setLat(m.tlat)
		}
	} else {
		m.setLat(m.Lat() - m.step)
		if m.Lat() < m.tlat {
			m.setLat(m.tlat)
		}
	}

	if m.tlon > m.Lon() {
		m.setLon(m.Lon() + m.step)
		if m.Lon() > m.tlon {
			m.setLon(m.tlon)
		}
	} else {
		m.setLon(m.Lon() - m.step)
		if m.Lon() < m.tlon {
			m.setLon(m.tlon)
		}
	}

	m.Update()
	time.Sleep(100 * time.Millisecond)
	go m.anim()
}

func (m *Map) wheel(event js.Value) {
	var delta float64
	if event.Get("deltaY").Int() < 0 {
		delta = m.zoomStep
	} else {
		delta = -m.zoomStep
	}

	m.SetPosition(m.zoom+delta, m.lat, m.lon)
}

func (m *Map) onKeyDown(event js.Value) {
	if event == js.Undefined() {
		event = js.Global().Get("window").Get("event")
	}

	if event == js.Undefined() {
		return
	}

	if !m.arrowDown {
		m.tlat = m.lat
		m.tlon = m.lon
	}
	switch event.Get("keyCode").Int() {
	case 38: // up
		m.tlat += 0.005
	case 40: // down
		m.tlat -= 0.005
	case 37: // left
		m.tlon -= 0.005
	case 39: // right
		m.tlon += 0.005
	default:
		return
	}

	m.arrowDown = true

	go m.anim()
}

func (m *Map) onKeyUp(event js.Value) {
	m.arrowDown = false
}

func (m *Map) onMouseDown(event js.Value) {
	if touches := event.Get("touches"); touches != js.Undefined() {
		event = touches.Index(0)
	}
	m.mouseStartX = event.Get("pageX").Int()
	m.mouseStartY = event.Get("pageY").Int()
	m.mouseStartLat = m.Lat()
	m.mouseStartLon = m.Lon()
	m.mouseDown = true
}

func (m *Map) onMouseUp(event js.Value) {
	m.mouseDown = false
}

func (m *Map) onMouseMove(event js.Value) {
	if !m.mouseDown {
		return
	}

	if touches := event.Get("touches"); touches != js.Undefined() {
		event = touches.Index(0)
	}
	dx := m.mouseStartX - event.Get("pageX").Int()
	dy := m.mouseStartY - event.Get("pageY").Int()

	lat, lon := Move(int(m.Zoom()), m.mouseStartLat, m.mouseStartLon, dx, dy)
	m.SetPosition(m.Zoom(), lat, lon)
}

// TilesFromCenter gets the tiles required from the centre point
func (m *Map) TilesFromCenter(canvasWidth, canvasHeight int) map[string]*Tile {
	cx, cy := TileNum(int(m.zoom), m.lat, m.lon)

	tx := int(cx)
	ty := int(cy)

	px := float64(tx) - cx
	py := float64(ty) - cy

	iz := int(m.zoom)
	scale := 1 + (0.5 + (m.zoom - float64(iz)))
	dx := -int(px * TileWidth * scale)
	dy := -int(py * TileHeight * scale)

	tiles := map[string]*Tile{}

	requiredWidth := int(math.Ceil(float64(canvasWidth)/(TileWidth*scale))) + 2
	requiredHeight := int(math.Ceil(float64(canvasHeight)/(TileHeight*scale))) + 2

	startX := -(requiredWidth / 2)
	startY := -(requiredHeight / 2)
	endX := startX + requiredWidth
	endY := startY + requiredHeight
	for cx := startX; cx < endX; cx++ {
		for cy := startY; cy < endY; cy++ {
			t := &Tile{
				URL:   m.urlEr.URL(int(m.zoom), cx+tx, cy+ty),
				DX:    dx - (cx * int(TileWidth*scale)),
				DY:    dy - (cy * int(TileHeight*scale)),
				Scale: scale,
			}
			tiles[t.URL.String()] = t
		}
	}

	return tiles
}

func (m *Map) Update() {
	for _, r := range m.tileRenderers {
		width, height := r.Viewport()
		tiles := m.TilesFromCenter(int(width), int(height))
		r.RenderTiles(tiles)
	}
}
