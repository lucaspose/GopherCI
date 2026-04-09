package handler

type Handlers struct {
	User *UserHandler
	Auth *AuthHandler
	Job *JobHandler
	SshKey *SSHKeyHandler
}
