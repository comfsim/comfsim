package linnea

import (
	"github.com/genshinsim/gcsim/pkg/core/attacks"
	"github.com/genshinsim/gcsim/pkg/core/attributes"
	"github.com/genshinsim/gcsim/pkg/core/combat"
	"github.com/genshinsim/gcsim/pkg/core/event"
	"github.com/genshinsim/gcsim/pkg/core/glog"
	"github.com/genshinsim/gcsim/pkg/core/info"
	"github.com/genshinsim/gcsim/pkg/core/player/character"
	"github.com/genshinsim/gcsim/pkg/modifier"
	"github.com/genshinsim/gcsim/pkg/reactable"
)

const (
	a1Key                    = "linnea-a1"
	a4Key                    = "linnea-a4"
	lunarcrystallizeBonusKey = "linnea-lcr-bonus"
)

func (c *char) a1OnLumi(src int) {
	if c.Base.Ascension < 1 {
		return
	}
	c.a1Ticker(src)
}

func (c *char) a1Ticker(src int) {
	if c.skillSrc != src {
		return
	}
	if !c.StatusIsActive(skillStandardPower) && !c.StatusIsActive(skillSuperPower) {
		return
	}
	shred := 0.15
	if c.Core.Player.GetMoonsignLevel() >= 2 {
		shred += 0.15
	}
	// CircleR10H10 is modeled as radius 10 on the 2D combat plane.
	for _, e := range c.Core.Combat.EnemiesWithinArea(combat.NewCircleHitOnTarget(c.LumiPos(), nil, 10), nil) {
		e.AddResistMod(info.ResistMod{
			Base:  modifier.NewBaseWithHitlag(a1Key, 1.5*60),
			Ele:   attributes.Geo,
			Value: -shred,
		})
	}
	c.Core.Tasks.Add(func() { c.a1Ticker(src) }, 60)
}

func (c *char) a4Init() {
	if c.Base.Ascension < 4 {
		return
	}
	c.Core.Events.Subscribe(event.OnInitialize, func(args ...any) {
		c.a4Ticker()
	}, a4Key)
	c.Core.Events.Subscribe(event.OnCharacterSwap, func(args ...any) {
		c.a4Update()
	}, a4Key)
}

func (c *char) a4Ticker() {
	c.a4Update()
	c.Core.Tasks.Add(c.a4Ticker, 30)
}

func (c *char) a4Update() {
	if c.Base.Ascension < 4 {
		return
	}
	target := c.Core.Player.ActiveChar()
	if target.Moonsign == 0 {
		target = c.CharWrapper
	}
	m := make([]float64, attributes.EndStatType)
	m[attributes.EM] = c.TotalDef(true) * 0.05
	// The dump refreshes a 1s modifier on the selected recipient every 0.5s.
	// A previous recipient keeps its last application until that expires.
	target.AddStatMod(character.StatMod{
		Base:         modifier.NewBase(a4Key, 60),
		AffectedStat: attributes.EM,
		Extra:        true,
		Amount:       func() []float64 { return m },
	})
}

func (c *char) moonsignInit() {
	c.Core.Flags.Custom[reactable.LunarCrystallizeEnableKey] = 1

	c.Core.Events.Subscribe(event.OnEnemyHit, func(args ...any) {
		atk := args[1].(*info.AttackEvent)

		if atk.Info.AttackTag != attacks.AttackTagDirectLunarCrystallize {
			return
		}

		bonus := min(c.TotalDef(false)/100.0*0.007, 0.14)

		if c.Core.Flags.LogDebug {
			c.Core.Log.NewEvent("linnea adding lunar crystallize base damage", glog.LogCharacterEvent, c.Index()).Write("bonus", bonus)
		}

		atk.Info.BaseDmgBonus += bonus
	}, lunarcrystallizeBonusKey)

	c.Core.Events.Subscribe(event.OnSpecialReactionAttack, func(args ...any) {
		atk := args[1].(*info.AttackEvent)

		if atk.Info.AttackTag != attacks.AttackTagReactionLunarCrystallize {
			return
		}

		bonus := min(c.TotalDef(false)/100.0*0.007, 0.14)

		if c.Core.Flags.LogDebug {
			c.Core.Log.NewEvent("linnea adding lunar crystallize base damage", glog.LogCharacterEvent, c.Index()).Write("bonus", bonus)
		}

		atk.Info.BaseDmgBonus += bonus
	}, lunarcrystallizeBonusKey+"reaction")
}
