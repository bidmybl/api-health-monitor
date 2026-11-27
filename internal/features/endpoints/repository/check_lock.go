package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
)

const acquireCheckLockQuery = `
	SELECT pg_try_advisory_lock(
		hashtextextended(
			'api-health-monitor:endpoint-check:' || $1::text,
			0
		)
	);
`

const releaseCheckLockQuery = `
	SELECT pg_advisory_unlock(
		hashtextextended(
			'api-health-monitor:endpoint-check:' || $1::text,
			0
		)
	);
`

func (r *Repository) TryAcquireCheckLock(
	ctx context.Context,
	endpointID int64,
) (func(context.Context) error, bool, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("acquire database connection: %w", err)
	}

	var acquired bool
	err = conn.QueryRow(
		ctx,
		acquireCheckLockQuery,
		strconv.FormatInt(endpointID, 10),
	).Scan(&acquired)

	if err != nil {
		conn.Release()
		return nil, false, fmt.Errorf("acquire endpoint check lock: %w", err)
	}

	if !acquired {
		conn.Release()
		return nil, false, nil
	}

	var once sync.Once
	var releaseErr error

	release := func(releaseCtx context.Context) error {
		once.Do(func() {
			var unlocked bool
			err := conn.QueryRow(
				releaseCtx,
				releaseCheckLockQuery,
				strconv.FormatInt(endpointID, 10),
			).Scan(&unlocked)

			if err != nil {
				rawConn := conn.Hijack()
				closeErr := rawConn.Close(context.Background())

				releaseErr = errors.Join(
					fmt.Errorf("release endpoint check lock: %w", err),
				)

				if closeErr != nil {
					releaseErr = errors.Join(
						releaseErr,
						fmt.Errorf("close database connection: %w", closeErr),
					)
				}

				return
			}

			conn.Release()

			if !unlocked {
				releaseErr = fmt.Errorf("endpoint check lock was not held")
			}
		})

		return releaseErr
	}

	return release, true, nil
}
