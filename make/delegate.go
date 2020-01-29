package make

var (
	makeMap = map[string]model{}

	makeList = [...]makeType{
		{"stiebel", 0x0001, unitDefault},
	}
)

func GetClientByMakeAndModel(make string, model string) *CanClient {
	return nil
}
