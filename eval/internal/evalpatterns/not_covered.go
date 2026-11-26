package evalpatterns

// The generator's ranges (evalgen's cell space): a value outside one is
// listed as not covered rather than clamped (AC-2.8).
const (
	minLayers, maxLayers     = 3, 5
	minPackages, maxPackages = 2, 4
	minDepth, maxDepth       = 2, 5
)

func outside(value, low, high int) bool { return value < low || value > high }

// recordLayers adds the layer count and, when the policy orders a layer
// count the generator builds, each layer's packages under its role.
func (p *profile) recordLayers(shape layerShape) {
	if len(shape.packages) == 0 {
		return
	}
	p.Dimensions.Layers.add(len(shape.packages))
	roles, buildable := generatorRoles[len(shape.packages)]
	if shape.ordered && buildable {
		for i, count := range shape.packages {
			p.Roles[roles[i]] = roleStats{Packages: distribution{itoa(count): 1}}
		}
	}
	unrolled := shape.fromPolicy && !shape.ordered
	if unrolled || outside(len(shape.packages), minLayers, maxLayers) {
		p.addNotCovered("layers", 1)
	}
	p.addNotCovered("packages", countOutside(shape.packages, minPackages, maxPackages))
}

func (p *profile) recordDepths(depths []int) {
	for _, depth := range depths {
		p.Dimensions.Depth.add(depth)
	}
	p.addNotCovered("depth", countOutside(depths, minDepth, maxDepth))
}

func countOutside(values []int, low, high int) int {
	count := 0
	for _, value := range values {
		if outside(value, low, high) {
			count++
		}
	}
	return count
}

// notCoveredOrder fixes the list's order, so the profile is reproducible.
var notCoveredOrder = map[string]int{"depth": 0, "layers": 1, "packages": 2}

func (p *profile) addNotCovered(shape string, count int) {
	if count == 0 {
		return
	}
	item := notCovered{Shape: shape, Count: count}
	at := len(p.NotCovered)
	for i, existing := range p.NotCovered {
		if notCoveredOrder[existing.Shape] > notCoveredOrder[shape] {
			at = i
			break
		}
	}
	p.NotCovered = append(p.NotCovered[:at], append([]notCovered{item}, p.NotCovered[at:]...)...)
}
