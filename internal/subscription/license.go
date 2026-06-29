package subscription

type License struct {
	Key         string `json:"key" yaml:"key"`
	Product     string `json:"product" yaml:"product"`
	ProductName string `json:"productName" yaml:"productName"`
	RegDate     string `json:"regDate" yaml:"regDate"`
	NextDueDate string `json:"nextDueDate" yaml:"nextDueDate"`
	Status      Status `json:"status" yaml:"status"`
}

type Registry struct {
	Licenses []License `json:"licenses" yaml:"licenses"`
	Servers  []Server  `json:"servers" yaml:"servers"`
}
