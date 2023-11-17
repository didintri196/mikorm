package client

type (
	IClient interface {
		Command(cmd string) IClause
	}

	IClause interface {
		Where(model interface{}) IClause
		Do() IExec
	}

	IExec interface {
		Print(model interface{}) (err error)
	}
)
