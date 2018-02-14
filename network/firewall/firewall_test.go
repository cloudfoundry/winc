package firewall_test

import (
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"os/exec"

	"code.cloudfoundry.org/winc/network/firewall"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Firewall", func() {
	var (
		f *firewall.Firewall
	)

	BeforeEach(func() {
		var err error
		f, err = firewall.NewFirewall(firewallDLL)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(f.Close()).To(Succeed())
	})

	Describe("Create", func() {
		var rule firewall.Rule

		BeforeEach(func() {
			rule = firewall.Rule{
				Name:            randomFirewallName(),
				RemoteAddresses: "1.2.3.4",
			}
		})

		AfterEach(func() {
			Expect(f.DeleteRule(rule.Name)).To(Succeed())
			Expect(netshRuleExists(rule.Name)).To(BeFalse())
		})

		It("creates the rule", func() {
			Expect(f.CreateRule(rule)).To(Succeed())
			o, err := exec.Command("netsh", "advfirewall", "firewall", "show", "rule", fmt.Sprintf(`name="%s"`, rule.Name)).CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), string(o))
			Expect(showRule(rule.Name)).To(MatchRegexp(`Enabled:\s*Yes`))
		})

		Context("the rule has a direction", func() {
			Context("the direction is in", func() {
				BeforeEach(func() {
					rule.Direction = firewall.NET_FW_RULE_DIR_IN
				})

				It("creates a rule with the direction", func() {
					Expect(f.CreateRule(rule)).To(Succeed())
					Expect(showRule(rule.Name)).To(MatchRegexp(`Direction:\s*In`))
				})
			})

			Context("the direction is out", func() {
				BeforeEach(func() {
					rule.Direction = firewall.NET_FW_RULE_DIR_OUT
				})

				It("creates a rule with the direction", func() {
					Expect(f.CreateRule(rule)).To(Succeed())
					Expect(showRule(rule.Name)).To(MatchRegexp(`Direction:\s*Out`))
				})
			})
		})

		Context("the rule has an action", func() {
			Context("the action is allow", func() {
				BeforeEach(func() {
					rule.Action = firewall.NET_FW_ACTION_ALLOW
				})

				It("creates a rule with the action", func() {
					Expect(f.CreateRule(rule)).To(Succeed())
					Expect(showRule(rule.Name)).To(MatchRegexp(`Action:\s*Allow`))
				})
			})

			Context("the action is block", func() {
				BeforeEach(func() {
					rule.Action = firewall.NET_FW_ACTION_BLOCK
				})

				It("creates a rule with the action", func() {
					Expect(f.CreateRule(rule)).To(Succeed())
					Expect(showRule(rule.Name)).To(MatchRegexp(`Action:\s*Block`))
				})
			})
		})

		Context("the rule has a protocol", func() {
			Context("the protocol is tcp", func() {
				BeforeEach(func() {
					rule.Protocol = firewall.NET_FW_IP_PROTOCOL_TCP
				})

				It("creates a rule with the protocol", func() {
					Expect(f.CreateRule(rule)).To(Succeed())
					Expect(showRule(rule.Name)).To(MatchRegexp(`Protocol:\s*TCP`))
				})

				Context("the rule includes a local port range", func() {
					BeforeEach(func() {
						rule.LocalPorts = "80-90,100"
					})

					It("creates a rule with the local ports", func() {
						Expect(f.CreateRule(rule)).To(Succeed())
						Expect(showRule(rule.Name)).To(MatchRegexp(`LocalPort:\s*80-90,100`))
					})
				})

				Context("the rule includes a remote port range", func() {
					BeforeEach(func() {
						rule.RemotePorts = "580-590,5100"
					})

					It("creates a rule with the local ports", func() {
						Expect(f.CreateRule(rule)).To(Succeed())
						Expect(showRule(rule.Name)).To(MatchRegexp(`RemotePort:\s*580-590,5100`))
					})
				})
			})

			Context("the protocol is udp", func() {
				BeforeEach(func() {
					rule.Protocol = firewall.NET_FW_IP_PROTOCOL_UDP
				})

				It("creates a rule with the protocol", func() {
					Expect(f.CreateRule(rule)).To(Succeed())
					Expect(showRule(rule.Name)).To(MatchRegexp(`Protocol:\s*UDP`))
				})

				Context("the rule includes a local port range", func() {
					BeforeEach(func() {
						rule.LocalPorts = "80-90,100"
					})

					It("creates a rule with the local ports", func() {
						Expect(f.CreateRule(rule)).To(Succeed())
						Expect(showRule(rule.Name)).To(MatchRegexp(`LocalPort:\s*80-90,100`))
					})
				})

				Context("the rule includes a remote port range", func() {
					BeforeEach(func() {
						rule.RemotePorts = "580-590,5100"
					})

					It("creates a rule with the local ports", func() {
						Expect(f.CreateRule(rule)).To(Succeed())
						Expect(showRule(rule.Name)).To(MatchRegexp(`RemotePort:\s*580-590,5100`))
					})
				})
			})

			Context("the protocol is icmp", func() {
				BeforeEach(func() {
					rule.Protocol = firewall.NET_FW_IP_PROTOCOL_ICMP
				})

				It("creates a rule with the protocol", func() {
					Expect(f.CreateRule(rule)).To(Succeed())
					Expect(showRule(rule.Name)).To(MatchRegexp(`Protocol:\s*ICMP`))
				})
			})

			Context("the protocol is any", func() {
				BeforeEach(func() {
					rule.Protocol = firewall.NET_FW_IP_PROTOCOL_ANY
				})

				It("creates a rule with the protocol", func() {
					Expect(f.CreateRule(rule)).To(Succeed())
					Expect(showRule(rule.Name)).To(MatchRegexp(`Protocol:\s*Any`))
				})
			})
		})

		Context("the rule contains local addresses", func() {
			BeforeEach(func() {
				rule.LocalAddresses = "1.2.3.4,8.8.4.4-8.8.8.8"
			})

			It("creates a rule with the local addresses", func() {
				Expect(f.CreateRule(rule)).To(Succeed())
				Expect(showRule(rule.Name)).To(MatchRegexp(`LocalIP:\s*1\.2\.3\.4\/32,8\.8\.4\.4-8\.8\.8\.8`))
			})
		})

		Context("the rule contains remote addresses", func() {
			BeforeEach(func() {
				rule.RemoteAddresses = "5.6.7.8,9.9.9.9-10.10.10.10"
			})

			It("creates a rule with the local addresses", func() {
				Expect(f.CreateRule(rule)).To(Succeed())
				Expect(showRule(rule.Name)).To(MatchRegexp(`RemoteIP:\s*5\.6\.7\.8\/32,9\.9\.9\.9-10\.10\.10\.10`))
			})
		})
	})

	Describe("RuleExists", func() {
		Context("the rule exists", func() {
			var name string

			BeforeEach(func() {
				name = randomFirewallName()
				rule := firewall.Rule{
					Name:           name,
					LocalAddresses: "9.10.11.12",
				}

				Expect(f.CreateRule(rule)).To(Succeed())
			})

			It("returns true", func() {
				Expect(f.RuleExists(name)).To(BeTrue())
			})
		})

		Context("the rule does not exist", func() {
			It("returns false", func() {
				Expect(f.RuleExists("no-way-this-is-an-actual-firewall-rule")).To(BeFalse())
			})
		})
	})

	Describe("DeleteRule", func() {
		var name string

		Context("a rule by that name exists", func() {
			BeforeEach(func() {
				name = randomFirewallName()
				rule := firewall.Rule{
					Name:            name,
					RemoteAddresses: "1.2.3.4",
				}
				Expect(f.CreateRule(rule)).To(Succeed())
			})

			It("deletes the rule", func() {
				Expect(f.DeleteRule(name)).To(Succeed())
				Expect(netshRuleExists(name)).To(BeFalse())
			})
		})

		Context("a rule by that name does not exist", func() {
			It("returns success", func() {
				Expect(f.DeleteRule("no-way-this-is-an-actual-firewall-rule")).To(Succeed())
			})
		})

		Context("multiple rules by that name exist", func() {
			BeforeEach(func() {
				name = randomFirewallName()
				rule := firewall.Rule{
					Name:            name,
					RemoteAddresses: "1.2.3.4",
				}
				Expect(f.CreateRule(rule)).To(Succeed())

				rule = firewall.Rule{
					Name:            name,
					RemoteAddresses: "5.6.7.8",
				}
				Expect(f.CreateRule(rule)).To(Succeed())
				Expect(showRule(rule.Name)).To(MatchRegexp(`RemoteIP:\s*1\.2\.3\.4\/32`))
				Expect(showRule(rule.Name)).To(MatchRegexp(`RemoteIP:\s*5\.6\.7\.8\/32`))
			})

			It("deletes all of the rules", func() {
				Expect(f.DeleteRule(name)).To(Succeed())
				Expect(netshRuleExists(name)).To(BeFalse())
			})
		})
	})
})

func randomFirewallName() string {
	max := big.NewInt(math.MaxInt64)
	r, err := rand.Int(rand.Reader, max)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	return fmt.Sprintf("firewall-%d", r.Int64())
}

func showRule(name string) string {
	o, err := exec.Command("netsh", "advfirewall", "firewall", "show", "rule", fmt.Sprintf(`name="%s"`, name)).CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), string(o))
	return string(o)
}

func netshRuleExists(name string) bool {
	o, err := exec.Command("netsh", "advfirewall", "firewall", "show", "rule", fmt.Sprintf(`name="%s"`, name)).CombinedOutput()
	if err != nil {
		Expect(string(o)).To(ContainSubstring("No rules match the specified criteria."))
		return false
	}
	return true
}
