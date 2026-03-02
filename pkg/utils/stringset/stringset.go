package stringset

type StringSet map[string]struct{}

func (ss StringSet) Add(s string) StringSet {
	ss[s] = struct{}{}
	return ss
}

func (ss StringSet) Contains(s string) bool {
	_, ok := ss[s]
	return ok
}
