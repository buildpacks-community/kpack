package v1alpha2

type BuildPriority int

var (
	BuildPriorityNone      = BuildPriority(0)
	BuildPriorityLow       = BuildPriority(1)
	BuildPriorityHigh      = BuildPriority(1000)
	BuildPriorityClassHigh = "kpack-build-high-priority"
	BuildPriorityClassLow  = "kpack-build-low-priority"
)

var PriorityClasses = map[BuildPriority]string{
	BuildPriorityNone: "",
	BuildPriorityLow:  BuildPriorityClassLow,
	BuildPriorityHigh: BuildPriorityClassHigh,
}

func (p BuildPriority) PriorityClass() string {
	return PriorityClasses[p]
}
