package draw

import (
	"fmt"
	"strings"
	"sync"

	"github.com/tdewolff/canvas"
	"golang.org/x/image/font/gofont/goregular"
)

type FontData struct {
	Family string
	Bytes  []byte
}

// FontRegistry owns immutable configured font families and lazily resolved
// system families. Canvas types never leave the internal draw package.
type FontRegistry struct {
	mu       sync.Mutex
	families map[string]*canvas.FontFamily
	resolved map[string]*canvas.FontFamily
	fallback *canvas.FontFamily
	retained [][]byte
}

func NewFontRegistry(fonts []FontData) (*FontRegistry, error) {
	registry := &FontRegistry{
		families: make(map[string]*canvas.FontFamily, len(fonts)),
		resolved: make(map[string]*canvas.FontFamily),
		retained: make([][]byte, 0, len(fonts)+1),
	}
	for _, source := range fonts {
		name := strings.TrimSpace(source.Family)
		if name == "" {
			return nil, fmt.Errorf("font family must not be empty")
		}
		if len(source.Bytes) == 0 {
			return nil, fmt.Errorf("font %q has no data", name)
		}
		key := strings.ToLower(name)
		if _, exists := registry.families[key]; exists {
			return nil, fmt.Errorf("font family %q was provided more than once", name)
		}
		data := append([]byte(nil), source.Bytes...)
		family := canvas.NewFontFamily(name)
		if err := family.LoadFont(data, 0, canvas.FontRegular); err != nil {
			return nil, fmt.Errorf("load font %q: %w", name, err)
		}
		registry.families[key] = family
		registry.retained = append(registry.retained, data)
	}

	fallbackData := append([]byte(nil), goregular.TTF...)
	registry.fallback = canvas.NewFontFamily("miq-fallback")
	if err := registry.fallback.LoadFont(fallbackData, 0, canvas.FontRegular); err != nil {
		return nil, fmt.Errorf("load embedded fallback font: %w", err)
	}
	registry.retained = append(registry.retained, fallbackData)
	return registry, nil
}

func (r *FontRegistry) Has(family string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.families[strings.ToLower(strings.TrimSpace(family))]
	return ok
}

func (r *FontRegistry) RegisterSystem(family string) bool {
	name := strings.TrimSpace(family)
	if name == "" {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := strings.ToLower(name)
	if _, ok := r.families[key]; ok {
		return true
	}
	loaded := canvas.NewFontFamily(name)
	if err := loaded.LoadSystemFont(name, canvas.FontRegular); err != nil {
		return false
	}
	r.families[key] = loaded
	r.resolved = make(map[string]*canvas.FontFamily)
	return true
}

func (r *FontRegistry) RegisterBytes(family string, data []byte) error {
	name := strings.TrimSpace(family)
	if name == "" {
		return fmt.Errorf("font family must not be empty")
	}
	if len(data) == 0 {
		return fmt.Errorf("font %q has no data", name)
	}
	copyOfData := append([]byte(nil), data...)
	loaded := canvas.NewFontFamily(name)
	if err := loaded.LoadFont(copyOfData, 0, canvas.FontRegular); err != nil {
		return fmt.Errorf("load font %q: %w", name, err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.families[strings.ToLower(name)] = loaded
	r.retained = append(r.retained, copyOfData)
	r.resolved = make(map[string]*canvas.FontFamily)
	return nil
}

func (r *FontRegistry) resolve(stack string) *canvas.FontFamily {
	r.mu.Lock()
	defer r.mu.Unlock()
	if family, ok := r.resolved[stack]; ok {
		return family
	}
	for _, candidate := range strings.Split(stack, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if family, ok := r.families[strings.ToLower(candidate)]; ok {
			r.resolved[stack] = family
			return family
		}
		family := canvas.NewFontFamily(candidate)
		if err := family.LoadSystemFont(candidate, canvas.FontRegular); err == nil {
			r.resolved[stack] = family
			return family
		}
	}
	r.resolved[stack] = r.fallback
	return r.fallback
}
