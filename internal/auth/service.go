package auth

func (s *Service) GetSecret() string {
	return s.secret
}