package evaldetect

// bypassLayers splits the policy's layers around the required layer R: a
// layer is upstream when it may import R but R may not import it, and
// downstream when R may import it but it may not import R. A layer that is
// both or neither takes no part in the bypass search.
func (p Policy) bypassLayers() (upstream, downstream map[string]bool) {
	upstream, downstream = map[string]bool{}, map[string]bool{}
	required := p.requiredLayer
	for _, layer := range p.layers {
		if layer.Name == required {
			continue
		}
		intoRequired := !p.forbidden[[2]string{layer.Name, required}]
		fromRequired := !p.forbidden[[2]string{required, layer.Name}]
		switch {
		case intoRequired && !fromRequired:
			upstream[layer.Name] = true
		case fromRequired && !intoRequired:
			downstream[layer.Name] = true
		}
	}
	return upstream, downstream
}
