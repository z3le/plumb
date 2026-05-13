package methods

type Server struct{}

func (s *Server) Start() error {
	return nil
}

func (s Server) Name() string {
	return "server"
}
