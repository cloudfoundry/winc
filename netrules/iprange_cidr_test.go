package netrules_test

import (
	"net"
	"strings"

	"code.cloudfoundry.org/winc/netrules"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("IPRangeToCIDRs", func() {
	DescribeTable("converting IP ranges to CIDR blocks",
		func(ipr netrules.IPRange, cidrs []string) {
			Expect(netrules.IPRangeToCIDRs(ipr)).To(Equal(cidrs))
		},

		Entry("0.0.0.0-0.0.0.0", ipRange("0.0.0.0-0.0.0.0"), []string{"0.0.0.0/32"}),
		Entry("0.0.0.1-0.0.0.2", ipRange("0.0.0.1-0.0.0.2"), []string{"0.0.0.1/32", "0.0.0.2/32"}),
		Entry("1.1.1.1-1.1.1.1", ipRange("1.1.1.1-1.1.1.1"), []string{"1.1.1.1/32"}),
		Entry("255.255.255.255-255.255.255.255", ipRange("255.255.255.255-255.255.255.255"), []string{"255.255.255.255/32"}),
		Entry("0.0.0.0-255.255.255.255", ipRange("0.0.0.0-255.255.255.255"), []string{"0.0.0.0/0"}),

		Entry("192.168.0.10-192.168.0.250", ipRange("192.168.0.10-192.168.0.250"),
			[]string{"192.168.0.10/31",
				"192.168.0.12/30",
				"192.168.0.16/28",
				"192.168.0.32/27",
				"192.168.0.64/26",
				"192.168.0.128/26",
				"192.168.0.192/27",
				"192.168.0.224/28",
				"192.168.0.240/29",
				"192.168.0.248/31",
				"192.168.0.250/32"},
		),

		Entry("1.2.3.4-5.6.7.8", ipRange("1.2.3.4-5.6.7.8"),
			[]string{
				"1.2.3.4/30",
				"1.2.3.8/29",
				"1.2.3.16/28",
				"1.2.3.32/27",
				"1.2.3.64/26",
				"1.2.3.128/25",
				"1.2.4.0/22",
				"1.2.8.0/21",
				"1.2.16.0/20",
				"1.2.32.0/19",
				"1.2.64.0/18",
				"1.2.128.0/17",
				"1.3.0.0/16",
				"1.4.0.0/14",
				"1.8.0.0/13",
				"1.16.0.0/12",
				"1.32.0.0/11",
				"1.64.0.0/10",
				"1.128.0.0/9",
				"2.0.0.0/7",
				"4.0.0.0/8",
				"5.0.0.0/14",
				"5.4.0.0/15",
				"5.6.0.0/22",
				"5.6.4.0/23",
				"5.6.6.0/24",
				"5.6.7.0/29",
				"5.6.7.8/32"},
		),
	)
})

func ipRange(r string) netrules.IPRange {
	a := strings.Split(r, "-")
	return netrules.IPRange{
		Start: net.ParseIP(a[0]),
		End:   net.ParseIP(a[1]),
	}
}
