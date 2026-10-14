package document

import "nimbleterminal.dev/nt3d"

// SetScene replaces the viewer's mesh and explicitly normalizes it to the
// two-unit world used by Gloss's named views and camera-distance options.
// Scene takes its own geometry snapshot, so recoloring the document cannot
// mutate a frame already being rendered.
func (m *Mesh) SetScene(scene *nt3d.Scene) error {
	node, err := scene.SetGeometry("mesh", m.geometry)
	if err != nil {
		return err
	}
	transform, _, _ := m.geometry.Bounds.Normalization()
	return scene.SetTransform(node, transform)
}

func (m *Mesh) Scene() (*nt3d.Scene, error) {
	scene := nt3d.NewScene()
	if err := m.SetScene(scene); err != nil {
		return nil, err
	}
	return scene, nil
}
