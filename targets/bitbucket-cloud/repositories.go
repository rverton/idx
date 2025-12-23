package bitbucketcloud

type Repository struct {
	Name string
	Slug string
	UUID string
}

func (c *APIClient) Repositories() []Repository {

	// make a request to /repositories endpoint

	return []Repository{}

}
