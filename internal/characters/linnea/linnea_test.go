package linnea

import (
	"fmt"
	"math"
	"strings"
	"testing"

	_ "github.com/genshinsim/gcsim/internal/characters/barbara"
	_ "github.com/genshinsim/gcsim/internal/characters/bennett"
	_ "github.com/genshinsim/gcsim/internal/characters/xingqiu"
	"github.com/genshinsim/gcsim/pkg/avatar"
	"github.com/genshinsim/gcsim/pkg/core"
	"github.com/genshinsim/gcsim/pkg/core/attacks"
	"github.com/genshinsim/gcsim/pkg/core/attributes"
	"github.com/genshinsim/gcsim/pkg/core/combat"
	"github.com/genshinsim/gcsim/pkg/core/event"
	"github.com/genshinsim/gcsim/pkg/core/info"
	"github.com/genshinsim/gcsim/pkg/core/keys"
	"github.com/genshinsim/gcsim/pkg/core/player/character"
	"github.com/genshinsim/gcsim/pkg/enemy"
	"github.com/genshinsim/gcsim/pkg/modifier"
	"github.com/genshinsim/gcsim/pkg/reactable"
	"github.com/genshinsim/gcsim/pkg/testhelper"
)

func init() { testhelper.RegisterTestWeapon() }

func testCore(t *testing.T, cons int, team ...keys.Char) (*core.Core, *char, *enemy.Enemy) {
	t.Helper()
	c, err := core.New(core.Opt{Seed: 1, IgnoreBurstEnergy: true})
	if err != nil {
		t.Fatal(err)
	}
	c.Combat.SetPlayer(avatar.New(c, info.Point{}, 1))
	target := enemy.New(c, info.EnemyProfile{
		Level: 100, Resist: make(map[attributes.Element]float64),
		Pos: info.Coord{X: 0, Y: 0, R: 1},
	})
	c.Combat.AddEnemy(target)
	c.Combat.DefaultTarget = target.Key()
	var linnea *char
	for _, key := range team {
		p := testhelper.DefaultProfile(key, testhelper.TestWeaponKey)
		p.Base.Ascension = 6
		p.Talents = info.TalentProfile{Attack: 10, Skill: 10, Burst: 10}
		if key == keys.Linnea {
			p.Base.Cons = cons
		}
		idx, err := c.AddChar(p)
		if err != nil {
			t.Fatal(err)
		}
		if key == keys.Linnea {
			linnea = c.Player.Chars()[idx].Character.(*char)
		}
	}
	if linnea == nil {
		t.Fatal("Linnea missing from test team")
	}
	c.Player.SetActive(linnea.Index())
	if err := c.Init(); err != nil {
		t.Fatal(err)
	}
	return c, linnea, target
}

func tickTo(t *testing.T, c *core.Core, end int) {
	t.Helper()
	for c.F < end {
		c.F++
		if err := c.Tick(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestC3BoostsSkill(t *testing.T) {
	_, l, _ := testCore(t, 3, keys.Linnea)
	t.Logf("C3, configured talents 10/10/10: actual levels=%d/%d/%d", l.TalentLvlAttack()+1, l.TalentLvlSkill()+1, l.TalentLvlBurst()+1)
	if l.TalentLvlAttack() != 9 || l.TalentLvlSkill() != 12 {
		t.Error("dump C3 upgrades the elemental skill by 3 levels; normal attack should stay at 10")
	}
}

func TestSkillContinuesAfterBurst(t *testing.T) {
	for _, burst := range []bool{false, true} {
		label := "E_only_control"
		if burst {
			label = "E_then_Q"
		}
		t.Run(label, func(t *testing.T) {
			c, l, _ := testCore(t, 0, keys.Linnea)
			before, after := 0, 0
			c.Events.Subscribe(event.OnEnemyDamage, func(args ...any) {
				a := args[1].(*info.AttackEvent)
				if strings.HasPrefix(a.Info.Abil, "Lumi ") {
					if c.F <= 400 {
						before++
					} else {
						after++
					}
				}
			}, "audit-attacks")
			if _, err := l.Skill(nil); err != nil {
				t.Fatal(err)
			}
			tickTo(t, c, 400)
			if burst {
				if _, err := l.Burst(nil); err != nil {
					t.Fatal(err)
				}
			}
			tickTo(t, c, 1000)
			t.Logf("Lumi hits: frames 1-400=%d; frames 401-1000=%d", before, after)
			if before == 0 {
				t.Fatal("invalid control: Lumi never attacked")
			}
			if after == 0 {
				t.Error("Lumi stopped attacking despite an active summon duration")
			}
		})
	}
}

func TestC2TeamOrder(t *testing.T) {
	teams := []struct {
		name  string
		chars []keys.Char
	}{
		{"eligible_first_control", []keys.Char{keys.Linnea, keys.Barbara, keys.Bennett}},
		{"pyro_first", []keys.Char{keys.Bennett, keys.Linnea, keys.Barbara}},
	}
	for _, tc := range teams {
		t.Run(tc.name, func(t *testing.T) {
			c, _, target := testCore(t, 2, tc.chars...)
			previous := make([]float64, len(c.Player.Chars()))
			for i, w := range c.Player.Chars() {
				previous[i] = w.Stat(attributes.CD)
			}
			c.Events.Emit(event.OnMoondriftHarmony, target, &info.AttackEvent{})
			for i, w := range c.Player.Chars() {
				delta := w.Stat(attributes.CD) - previous[i]
				t.Logf("%s crit-damage bonus=%g", w.Base.Key.String(), delta)
				if w.Base.Element == attributes.Geo || w.Base.Element == attributes.Hydro {
					if math.Abs(delta-0.4) > 1e-6 {
						t.Error("eligible teammate did not receive C2 +40% crit damage")
					}
				}
			}
		})
	}
}

func TestC6SkillGrants18Stacks(t *testing.T) {
	_, l, _ := testCore(t, 6, keys.Linnea)
	if _, err := l.Skill(nil); err != nil {
		t.Fatal(err)
	}
	t.Logf("C6 stacks immediately after E: %d", l.c1Stacks)
	if l.c1Stacks != 18 {
		t.Error("dump routes skill activation through C6's fill-to-18 branch")
	}
}

func TestHealingFormula(t *testing.T) {
	c, l, _ := testCore(t, 0, keys.Linnea)
	got := 0.0
	def := l.TotalDef(false)
	c.Events.Subscribe(event.OnHeal, func(args ...any) {
		h := args[0].(*info.HealInfo)
		if strings.HasSuffix(h.Message, "(Initial)") {
			got = h.Src
		}
	}, "audit-initial-heal")
	if _, err := l.Burst(nil); err != nil {
		t.Fatal(err)
	}
	tickTo(t, c, 100)
	expected := 1694.95458984375 + 2.880000114440918*def
	t.Logf("level-10 initial healing before healing bonuses: DEF=%.6f got=%.6f expected=%.6f", def, got, expected)
	if math.Abs(got-expected) > 0.01 {
		t.Error("flat healing and DEF scaling are reversed")
	}
}

func TestPeriodicHealing(t *testing.T) {
	c, l, _ := testCore(t, 0, keys.Linnea, keys.Barbara)
	frames := []int{}
	offField := 0
	c.Events.Subscribe(event.OnHeal, func(args ...any) {
		h := args[0].(*info.HealInfo)
		if strings.HasSuffix(h.Message, "(Tick)") {
			if args[1].(int) == c.Player.Active() {
				frames = append(frames, c.F)
			} else {
				offField++
			}
		}
	}, "audit-periodic-heal")
	if _, err := l.Burst(nil); err != nil {
		t.Fatal(err)
	}
	tickTo(t, c, 900)
	t.Logf("periodic heal frames for active character=%v; off-field periodic heals=%d", frames, offField)
	// The modifier lasts 12 seconds at a 1-second interval. This assertion
	// deliberately does not depend on the engine's exact expiry-frame convention.
	if len(frames) == 0 || frames[len(frames)-1] < initialHeal+10*60 {
		t.Error("periodic healing stops well before the dump's 12-second duration")
	}
	if offField != 0 {
		t.Error("dump periodic healing targets on-field avatars, but bench characters are healed")
	}
}

func TestC2EmitsResonance(t *testing.T) {
	c, l, _ := testCore(t, 2, keys.Linnea, keys.Barbara)
	// Supply the second Moonsign contribution without adding unrelated passives.
	c.Player.Chars()[1].Moonsign = 1
	count := 0
	c.Events.Subscribe(event.OnMoondriftHarmony, func(args ...any) { count++ }, "audit-resonance")
	l.c2TriggerMoonDrift(&info.AttackEvent{Info: info.AttackInfo{ActorIndex: l.Index()}})
	tickTo(t, c, 300)
	t.Logf("C2 forced resonance: OnMoondriftHarmony events=%d", count)
	if count == 0 {
		t.Error("C2 queues reaction damage but omits the resonance event used by C1/C2/C4")
	}
}

func TestUltimateFormStartsA1(t *testing.T) {
	for _, recast := range []bool{false, true} {
		label := "single_E_control"
		if recast {
			label = "ultimate_form"
		}
		t.Run(label, func(t *testing.T) {
			c, l, target := testCore(t, 0, keys.Linnea)
			if _, err := l.Skill(nil); err != nil {
				t.Fatal(err)
			}
			if recast {
				for _, frame := range []int{20, 40, 60} {
					tickTo(t, c, frame)
					if _, err := l.Skill(nil); err != nil {
						t.Fatal(err)
					}
				}
			}
			tickTo(t, c, 300)
			active := target.ResistModIsActive(a1Key)
			t.Logf("A1 Geo resistance reduction active at frame 300: %v", active)
			if !active {
				t.Error("Lumi is active but the A1 resistance-reduction ticker never started")
			}
		})
	}
}

func TestC1DirectHitConsumption(t *testing.T) {
	for _, cons := range []int{1, 6} {
		for _, million := range []bool{false, true} {
			for _, stacks := range []int{0, 1, 3, 18} {
				t.Run(fmt.Sprintf("C%d/million=%v/stacks=%d", cons, million, stacks), func(t *testing.T) {
					c, l, _ := testCore(t, cons, keys.Linnea)
					l.AddStatus(c1Key, 600, true)
					l.c1Stacks = stacks
					limit, ratio := 1, 0.75
					abil := skillOverdriveAbil
					if million {
						limit, ratio, abil = 5, 1.5, skillMillionAbil
					}
					if cons == 6 {
						limit *= 2
						ratio *= 0.75
					}
					used := min(stacks, limit)
					want := float64(used) * ratio * l.TotalDef(false)
					got := -1.0
					c.Events.Subscribe(event.OnEnemyHit, func(args ...any) {
						got = args[1].(*info.AttackEvent).Info.FlatDmg
					}, "test-bonus")
					ai := info.AttackInfo{
						ActorIndex: l.Index(), Abil: abil, Element: attributes.Geo,
						AttackTag: attacks.AttackTagDirectLunarCrystallize, IgnoreDefPercent: 1,
					}
					c.QueueAttack(ai, combat.NewCircleHitOnTarget(info.Point{}, nil, 5), 0, 0)
					if math.Abs(got-want) > 1e-6 || l.c1Stacks != stacks-used {
						t.Fatalf("bonus=%g want=%g; stacks=%d want=%d", got, want, l.c1Stacks, stacks-used)
					}
				})
			}
		}
	}
}

// This tests the agreed multi-target model, not independent proof of the
// native game's still-obfuscated ModifyDamageCountMixin behavior.
func TestC1MultipleTargetsAndSameFrameHits(t *testing.T) {
	for _, stacks := range []int{1, 18} {
		t.Run(fmt.Sprintf("stacks=%d", stacks), func(t *testing.T) {
			c, l, _ := testCore(t, 1, keys.Linnea)
			for _, x := range []float64{1, 2} {
				c.Combat.AddEnemy(enemy.New(c, info.EnemyProfile{
					Level:  100,
					Resist: make(map[attributes.Element]float64), Pos: info.Coord{X: x, R: 0.1},
				}))
			}
			l.AddStatus(c1Key, 600, true)
			l.c1Stacks = stacks
			var bonuses []float64
			c.Events.Subscribe(event.OnEnemyHit, func(args ...any) {
				bonuses = append(bonuses, args[1].(*info.AttackEvent).Info.FlatDmg)
			}, "test-aoe")
			ai := info.AttackInfo{
				ActorIndex: l.Index(), Abil: "test", Element: attributes.Geo,
				AttackTag: attacks.AttackTagDirectLunarCrystallize, IgnoreDefPercent: 1,
			}
			// A miss consumes nothing.
			c.QueueAttack(ai, combat.NewCircleHitOnTarget(info.Point{X: 100}, nil, 1), 0, 0)
			if l.c1Stacks != stacks {
				t.Fatal("miss consumed stacks")
			}
			for range 2 {
				c.QueueAttack(ai, combat.NewCircleHitOnTarget(info.Point{}, nil, 5), 0, 0)
			}
			if len(bonuses) != 6 || l.c1Stacks != max(0, stacks-2) {
				t.Fatalf("bonuses=%v stacks=%d", bonuses, l.c1Stacks)
			}
			for i, got := range bonuses {
				want := 0.75 * l.TotalDef(false)
				if stacks == 1 && i >= 3 {
					want = 0
				}
				if math.Abs(got-want) > 1e-6 {
					t.Errorf("target hit %d: bonus=%g want=%g", i, got, want)
				}
			}
		})
	}
}

func TestC1MoondriftContributors(t *testing.T) {
	for _, cons := range []int{1, 6} {
		for count := 1; count <= 4; count++ {
			t.Run(fmt.Sprintf("C%d/contributors=%d", cons, count), func(t *testing.T) {
				c, l, target := testCore(t, cons, keys.Linnea, keys.Barbara, keys.Bennett, keys.Xingqiu)
				l.AddStatus(c1Key, 600, true)
				l.c1Stacks = 18
				perHit, ratio := 1, 0.75
				if cons == 6 {
					perHit, ratio = 2, 1.125
				}
				var contributors [info.MaxChars]bool
				for i := range count {
					contributors[i] = true
				}
				observed := 0
				c.Events.Subscribe(event.OnSpecialReactionAttack, func(args ...any) {
					a := args[1].(*info.AttackEvent)
					observed++
					if math.Abs(a.Info.FlatDmg-ratio*l.TotalDef(false)) > 1e-6 {
						t.Errorf("contributor %d received incorrect C1 flat bonus: %g", a.Info.ActorIndex, a.Info.FlatDmg)
					}
				}, "test-contributors")
				reactable.DoLCrAttackWithContrib(contributors, target, c, l.Index())
				tickTo(t, c, 60)
				if observed != count*3 || l.c1Stacks != 18-3*perHit {
					t.Fatalf("contribution events=%d stacks=%d", observed, l.c1Stacks)
				}
			})
		}
	}
}

func TestA4RefreshAndExtraStats(t *testing.T) {
	c, l, target := testCore(t, 4, keys.Linnea, keys.Barbara, keys.Bennett)
	base := l.TotalDef(true)
	baseEM := l.NonExtraStat(attributes.EM)
	checkEM := func(w *character.CharWrapper, want float64) {
		t.Helper()
		if got := w.Stat(attributes.EM); math.Abs(got-want) > 1e-6 {
			t.Fatalf("%s EM=%g want=%g at frame %d", w.Base.Key, got, want, c.F)
		}
	}
	checkEM(l.CharWrapper, baseEM+0.05*base)
	// Extra DEF must not enter this conversion; output EM must itself be extra.
	m := make([]float64, attributes.EndStatType)
	m[attributes.DEF] = 1000
	l.AddStatMod(character.StatMod{
		Base:         modifier.NewBase("test-extra-def", -1),
		AffectedStat: attributes.DEF, Extra: true, Amount: func() []float64 { return m },
	})
	l.AddStatMod(character.StatMod{
		Base:         modifier.NewBase("test-def", -1),
		AffectedStat: attributes.DEF, Amount: func() []float64 { return m },
	})
	tickTo(t, c, 29)
	checkEM(l.CharWrapper, baseEM+0.05*base)
	tickTo(t, c, 30)
	checkEM(l.CharWrapper, baseEM+0.05*(base+1000))
	if l.NonExtraStat(attributes.EM) != baseEM {
		t.Fatal("A4 leaked into non-extra EM")
	}
	// Swap refreshes immediately, and the previous recipient expires after 1s.
	c.Player.ByIndex(1).Moonsign = 1
	c.Player.SetActive(1)
	c.Events.Emit(event.OnCharacterSwap, 0, 1)
	barbara := c.Player.ActiveChar()
	checkEM(barbara, barbara.NonExtraStat(attributes.EM)+0.05*(base+1000))
	tickTo(t, c, 90)
	checkEM(l.CharWrapper, baseEM)
	// C4 must refresh A4 after increasing DEF, without waiting for the ticker.
	tickTo(t, c, 91)
	c.Events.Emit(event.OnMoondriftHarmony, target, &info.AttackEvent{})
	checkEM(barbara, barbara.NonExtraStat(attributes.EM)+0.05*l.TotalDef(true))
	c.Player.SetActive(2)
	c.Events.Emit(event.OnCharacterSwap, 1, 2)
	checkEM(l.CharWrapper, baseEM+0.05*l.TotalDef(true))
}

func TestA1RadiusTen(t *testing.T) {
	c, l, target := testCore(t, 0, keys.Linnea)
	for _, x := range []float64{9, 11} {
		c.Combat.AddEnemy(enemy.New(c, info.EnemyProfile{
			Level:  100,
			Resist: make(map[attributes.Element]float64), Pos: info.Coord{X: x, R: 0.01},
		}))
	}
	l.AddStatus(skillSuperPower, skillDur, false)
	l.a1OnLumi(l.skillSrc)
	if !target.ResistModIsActive(a1Key) {
		t.Fatal("primary target did not receive A1")
	}
	for _, e := range c.Combat.Enemies() {
		want := e.Pos().X < 10
		if e.(*enemy.Enemy).ResistModIsActive(a1Key) != want {
			t.Errorf("A1 at x=%g: want active=%v", e.Pos().X, want)
		}
	}
}
