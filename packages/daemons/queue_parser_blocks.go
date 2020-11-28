/*---------------------------------------------------------------------------------------------
 *  Copyright (c) IBAX. All rights reserved.
 *  See LICENSE in the project root for license information.
 *--------------------------------------------------------------------------------------------*/

package daemons

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/IBAX-io/go-ibax/packages/conf"
	"github.com/IBAX-io/go-ibax/packages/conf/syspar"
	"github.com/IBAX-io/go-ibax/packages/consts"
	"github.com/IBAX-io/go-ibax/packages/model"
	"github.com/IBAX-io/go-ibax/packages/utils"

	log "github.com/sirupsen/logrus"
)

/* Take the block from the queue. If this block has the bigger block id than the last block from our chain, then find the fork
 * If fork begins less then variables->rollback_blocks blocks ago, than
 *  - get the whole chain of blocks
 *  - roll back data from our blocks
 *  - insert the frontal data from a new chain
 *  - if there is no error, then roll back our data from the blocks
 *  - and insert new data
 *  - if there are errors, then roll back to the former data
 * */

// QueueParserBlocks parses and applies blocks from the queue
func QueueParserBlocks(ctx context.Context, d *daemon) error {
	if atomic.CompareAndSwapUint32(&d.atomic, 0, 1) {
		defer atomic.StoreUint32(&d.atomic, 0)
	} else {
		return nil
	}
	DBLock()
	defer DBUnlock()

	infoBlock := &model.InfoBlock{}
	_, err := infoBlock.Get()
	if err != nil {
		d.logger.WithFields(log.Fields{"type": consts.DBError, "error": err}).Error("getting info block")
		return err
	}
	queueBlock := &model.QueueBlock{}
	_, err = queueBlock.Get()
	if err != nil {
		d.logger.WithFields(log.Fields{"type": consts.DBError, "error": err}).Error("getting queue block")
		return err
	}
	if len(queueBlock.Hash) == 0 {
		d.logger.WithFields(log.Fields{"type": consts.NotFound}).Debug("queue block not found")
		return err
	}

	// check if the block gets in the rollback_blocks_1 limit
	if queueBlock.BlockID > infoBlock.BlockID+syspar.GetRbBlocks1() {
		queueBlock.DeleteOldBlocks()
		return utils.ErrInfo("rollback_blocks_1")
	}


	nodeHost, err := syspar.GetNodeHostByPosition(queueBlock.HonorNodeID)
	if err != nil {
		queueBlock.DeleteQueueBlockByHash()
		return utils.ErrInfo(err)
	}
	blockID := queueBlock.BlockID

	host := utils.GetHostPort(nodeHost)
	// update our chain till maxBlockID from the host
	return UpdateChain(ctx, d, host, blockID)
}