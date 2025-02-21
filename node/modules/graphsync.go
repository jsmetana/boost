package modules

import (
	"github.com/ipfs/go-graphsync"
	graphsyncimpl "github.com/ipfs/go-graphsync/impl"
	gsnet "github.com/ipfs/go-graphsync/network"
	"github.com/ipfs/go-graphsync/storeutil"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/fx"

	"github.com/filecoin-project/lotus/node/config"
	"github.com/filecoin-project/lotus/node/modules/dtypes"
	"github.com/filecoin-project/lotus/node/modules/helpers"
	"github.com/filecoin-project/lotus/node/repo"
)

// Graphsync creates a graphsync instance from the given loader and storer
func Graphsync(parallelTransfersForStorage uint64, parallelTransfersForRetrieval uint64) func(mctx helpers.MetricsCtx, lc fx.Lifecycle, r repo.LockedRepo, clientBs dtypes.ClientBlockstore, chainBs dtypes.ExposedBlockstore, h host.Host) (dtypes.Graphsync, error) {
	return func(mctx helpers.MetricsCtx, lc fx.Lifecycle, r repo.LockedRepo, clientBs dtypes.ClientBlockstore, chainBs dtypes.ExposedBlockstore, h host.Host) (dtypes.Graphsync, error) {
		graphsyncNetwork := gsnet.NewFromLibp2pHost(h)
		lsys := storeutil.LinkSystemForBlockstore(clientBs)

		gs := graphsyncimpl.New(helpers.LifecycleCtx(mctx, lc),
			graphsyncNetwork,
			lsys,
			graphsyncimpl.RejectAllRequestsByDefault(),
			graphsyncimpl.MaxInProgressIncomingRequests(parallelTransfersForStorage),
			graphsyncimpl.MaxInProgressOutgoingRequests(parallelTransfersForRetrieval),
			graphsyncimpl.MaxLinksPerIncomingRequests(config.MaxTraversalLinks),
			graphsyncimpl.MaxLinksPerOutgoingRequests(config.MaxTraversalLinks))
		chainLinkSystem := storeutil.LinkSystemForBlockstore(chainBs)
		err := gs.RegisterPersistenceOption("chainstore", chainLinkSystem)
		if err != nil {
			return nil, err
		}
		gs.RegisterIncomingRequestHook(func(p peer.ID, requestData graphsync.RequestData, hookActions graphsync.IncomingRequestHookActions) {
			_, has := requestData.Extension("chainsync")
			if has {
				// TODO: we should confirm the selector is a reasonable one before we validate
				// TODO: this code will get more complicated and should probably not live here eventually
				hookActions.ValidateRequest()
				hookActions.UsePersistenceOption("chainstore")
			}
		})
		gs.RegisterOutgoingRequestHook(func(p peer.ID, requestData graphsync.RequestData, hookActions graphsync.OutgoingRequestHookActions) {
			_, has := requestData.Extension("chainsync")
			if has {
				hookActions.UsePersistenceOption("chainstore")
			}
		})
		return gs, nil
	}
}
