package fakegithub

// Fixture returns the Fixture passed to NewServer.
func (s *Server) Fixture() *Fixture { return s.fixture }
