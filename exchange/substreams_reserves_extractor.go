package exchange

import (
	"encoding/json"
	"fmt"

	eth "github.com/streamingfast/eth-go"
	pbcodec "github.com/streamingfast/sparkle/pb/sf/ethereum/codec/v1"
	"github.com/streamingfast/substream-pancakeswap/state"
)

type ReservesExtractor struct {
	*SubstreamIntrinsics
}

// Map function can take one or more input objects, sync'd by the
// `Block` clock.  Because it depends on a `PairsState`, it needs to
// be run in `Process`, linearly.
func (p *ReservesExtractor) Map(block *pbcodec.Block, pairsState state.Reader) (reserves PCSReserveUpdates, err error) {
	for _, trx := range block.TransactionTraces {
		for _, log := range trx.Receipt.Logs {
			// perhaps we can optimize in a small local map, if we
			// found previously in this call, in the State or perhaps
			// we do that in the `GetLast()` stack, optimized
			// heuristics.
			addr := eth.Address(log.Address).Pretty()
			pairCnt, found := pairsState.GetLast("pair:" + addr)
			if !found {
				continue
			}

			ethLog := ssCodecLogToEthLog(log)
			if IsPairSyncEvent(ethLog) {
				ev, err := NewPairSyncEvent(ethLog, block, trx)
				if err != nil {
					return nil, fmt.Errorf("decoding PairSync event: %w", err)
				}

				var pair PCSPair
				if err := json.Unmarshal(pairCnt.Bytes(), &pair); err != nil {
					return nil, err
				}

				reserve0 := ConvertTokenToDecimal(ev.Reserve0, pair.Token0.Decimals)
				reserve1 := ConvertTokenToDecimal(ev.Reserve1, pair.Token1.Decimals)

				update := PCSReserveUpdate{
					PairAddress: eth.Address(log.Address).Pretty(),
					Reserve0:    floatToStr(reserve0),
					Reserve1:    floatToStr(reserve1),
					LogOrdinal:  uint64(log.BlockIndex),
				}

				///// OPTIONAL?
				// THESE TWO FIELDS COULD VERY WELL BE COMPUTED DOWNSTREAM, IT'S JUST A DIVISION
				// BETWEEN THE TWO FIELDS.  WE CONVEY ENOUGH, BECAUSE WE BLEND IN THE DECIMALS
				//
				// https://stackoverflow.com/questions/64257065/is-there-another-way-of-testing-if-a-big-int-is-0
				if len(ev.Reserve1.Bits()) == 0 {
					update.Token0Price = "0"
				} else {
					update.Token0Price = floatToStr(bf().Quo(reserve0, reserve1))
				}

				if len(ev.Reserve0.Bits()) == 0 {
					update.Token1Price = "0"
				} else {
					update.Token1Price = floatToStr(bf().Quo(reserve1, reserve0))
				}
				// END OPTIONAL?

				reserves = append(reserves, update)
			}
		}
	}
	return
}
