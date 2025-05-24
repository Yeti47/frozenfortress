package secrets

type SecretDto struct {
	Id         string
	UserId     string
	Name       string
	Value      string
	CreatedAt  string
	ModifiedAt string
}

type UpsertSecretRequest struct {
	SecretName  string
	SecretValue string
}

type CreateSecretResponse struct {
	SecretId string
}

type PaginatedSecretResponse struct {
	TotalCount int
	Page       int
	PageSize   int
	Secrets    []*SecretDto
}

type GetSecretsRequest struct {
	Name     string
	PageSize int
	Page     int
	SortBy   string
	SortAsc  bool
}
