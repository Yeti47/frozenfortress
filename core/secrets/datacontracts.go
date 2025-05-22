package secrets

type SecretDto struct {
	Id         string
	UserId     string
	Name       string
	Value      string
	CreatedAt  string
	ModifiedAt string
}

type CreateSecretRequest struct {
	SecretName  string
	SecretValue string
}

type CreateSecretResponse struct {
	SecretId string
}
