package cpu

func New() *CPU {
	return &CPU{}
}

func (c *CPU) Collect() error {
	return nil
}

func (c *CPU) Name() string {
	return "CPU"
}

func (c *CPU) Marshal() {

}

func (c *CPU) PrintDetail() {}

func (c *CPU) PrintBreif() {}
