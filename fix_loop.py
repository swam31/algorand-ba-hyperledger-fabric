import sys

with open("fabric/consenter/chain.go", "r") as f:
    code = f.read()

target = """		case msg, ok := <-c.transport.Incoming():
			if !ok {
				return
			}
			vote, err := ba.DecodeVote(msg)
			if err != nil {
				fmt.Printf(">>> [ALGORAND BA*] Failed to decode incoming vote: %v\\n", err)
				continue
			}
			c.node.AddVote(vote)"""

replacement = """		case msg, ok := <-c.transport.Incoming():
			if !ok {
				return
			}
			
			// 1. Try decoding as BA* vote
			vote, err := ba.DecodeVote(msg)
			if err == nil {
				c.node.AddVote(vote)
				continue
			}

			// 2. Try decoding as Envelope (forwarded transaction)
			env := &common.Envelope{}
			if err := proto.Unmarshal(msg, env); err == nil && env.Payload != nil {
				payload := &common.Payload{}
				if err := proto.Unmarshal(env.Payload, payload); err == nil && payload.Header != nil {
					ch := &common.ChannelHeader{}
					if err := proto.Unmarshal(payload.Header.ChannelHeader, ch); err == nil {
						isConfig := (ch.Type == int32(common.HeaderType_CONFIG) || ch.Type == int32(common.HeaderType_CONFIG_UPDATE))
						if isConfig {
							batch := c.support.BlockCutter().Cut()
							if len(batch) > 0 {
								pending = append(pending, pBatch{envs: batch})
							}
							pending = append(pending, pBatch{envs: []*common.Envelope{env}, isConf: true})
						} else {
							batches, _ := c.support.BlockCutter().Ordered(env)
							for _, batch := range batches {
								pending = append(pending, pBatch{envs: batch})
							}
						}
					}
				}
				continue
			}
			fmt.Printf(">>> [ALGORAND BA*] Failed to decode incoming msg as Vote or Envelope\\n")"""

if target in code:
    code = code.replace(target, replacement)
    with open("fabric/consenter/chain.go", "w") as f:
        f.write(code)
    print("Replaced!")
else:
    print("Target not found")
