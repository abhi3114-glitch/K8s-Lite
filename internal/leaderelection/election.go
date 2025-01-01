package leaderelection

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/abhigod/k8s-lite/internal/api"
	"github.com/abhigod/k8s-lite/internal/client"
)

type Config struct {
	LockName      string
	Identity      string
	LeaseDuration time.Duration
	RenewDeadline time.Duration
	RetryPeriod   time.Duration
	Callbacks     Callbacks
	Client        *client.Client
}

type Callbacks struct {
	OnStartedLeading func(context.Context)
	OnStoppedLeading func()
}

// RunOrDie blocks until a leader is elected and then runs the callback.
func RunOrDie(ctx context.Context, config Config) {
	le := &LeaderElector{config: config}
	le.Run(ctx)
}

type LeaderElector struct {
	config Config
}

func (le *LeaderElector) Run(ctx context.Context) {
	log.Printf("Attempting to acquire leader lease %s...", le.config.LockName)

	for {
		// 1. Try to acquire or renew
		if err := le.tryAcquireOrRenew(ctx); err != nil {
			log.Printf("Failed to acquire lease: %v", err)
			// Failed, wait and retry
		} else {
			// Success, we are leader
			log.Printf("Successfully acquired lease %s. Leading...", le.config.LockName)

			// Start the leader loop
			ctx, cancel := context.WithCancel(ctx)
			go le.config.Callbacks.OnStartedLeading(ctx)

			// Renew loop
			le.renewLoop(ctx)

			// If we exit renew loop, we lost leadership
			cancel()
			le.config.Callbacks.OnStoppedLeading()
			log.Printf("Lost leadership for %s", le.config.LockName)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(le.config.RetryPeriod):
			continue
		}
	}
}

func (le *LeaderElector) renewLoop(ctx context.Context) {
	ticker := time.NewTicker(le.config.RetryPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := le.tryAcquireOrRenew(ctx); err != nil {
				log.Printf("Failed to renew lease: %v", err)
				return // Lost lease
			}
		}
	}
}

func (le *LeaderElector) tryAcquireOrRenew(ctx context.Context) error {
	client := le.config.Client
	now := time.Now()

	lease, err := client.GetLease(ctx, le.config.LockName)
	if err != nil {
		return err
	}

	if lease == nil {
		// If not found, create it
		lease = &api.Lease{
			ObjectMeta: api.ObjectMeta{
				Name: le.config.LockName,
			},
			Spec: api.LeaseSpec{
				HolderIdentity:       &le.config.Identity,
				AcquireTime:          &now,
				RenewTime:            &now,
				LeaseDurationSeconds: int32Ptr(int32(le.config.LeaseDuration.Seconds())),
			},
		}
		return client.CreateLease(ctx, lease)
	}

	// Check if existing lease is valid
	if lease.Spec.HolderIdentity != nil {
		if *lease.Spec.HolderIdentity == le.config.Identity {
			// We are holder, renew
			lease.Spec.RenewTime = &now
			return client.UpdateLease(ctx, lease)
		}

		// Someone else holds it. Check expiration.
		if lease.Spec.RenewTime != nil {
			expireTime := lease.Spec.RenewTime.Add(time.Duration(*lease.Spec.LeaseDurationSeconds) * time.Second)
			if now.Before(expireTime) {
				// Valid lease held by other
				return fmt.Errorf("lease currently held by %s", *lease.Spec.HolderIdentity)
			}
		}
	}

	// Lease expired or empty, acquire it
	// We need to be careful with resource version (optimistic locking)
	// client.UpdateLease handles checking ResourceVersion if we passed it back from GetLease
	// (Our simple store/client might not enforce it strictly unless we implemented it fully in storage layer,
	// but standard mechanism is to send back ResourceVersion)

	lease.Spec.HolderIdentity = &le.config.Identity
	lease.Spec.AcquireTime = &now
	lease.Spec.RenewTime = &now
	lease.Spec.LeaseDurationSeconds = int32Ptr(int32(le.config.LeaseDuration.Seconds()))

	// Increment transitions if trackable

	return client.UpdateLease(ctx, lease)
}

func int32Ptr(i int32) *int32 { return &i }



