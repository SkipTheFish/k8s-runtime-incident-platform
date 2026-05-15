package operator

import (
	"context"
	"time"
	"fmt"
	obsv1 "observability-platform/api/v1"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const AlertPolicyFinalizer = "obs.io/alertpolicy-finalizer"

type AlertPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Repo   Repository
}


func (r *AlertPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var policy obsv1.AlertPolicy
	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// === 1. 处理删除逻辑 (izer) ===
	if !policy.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &policy)
	}

	// === 2. 确保 izer 存在 ===
	if !containsString(policy.Finalizers, AlertPolicyFinalizer) {
		policy.Finalizers = append(policy.Finalizers, AlertPolicyFinalizer)
		if err := r.Update(ctx, &policy); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// === 3. 同步到数据库 ===
	dbID, err := r.Repo.UpsertAlertPolicy(ctx, &policy)
	if err != nil {
		fmt.Printf("❌ Database Sync Failed: %v\n", err)
		r.updateStatus(ctx, &policy, false, 0, err.Error())
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}

	// === 4. 同步到 Prometheus (核心修正：移到这里) ===
	if err := r.reconcilePrometheusRule(ctx, &policy); err != nil {
		fmt.Printf("❌ PrometheusRule Sync Failed: %v\n", err)
		r.updateStatus(ctx, &policy, false, dbID, err.Error())
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}

	// === 5. 更新成功状态 ===
	return ctrl.Result{}, r.updateStatus(ctx, &policy, true, dbID, "")
}

// 抽取出来的 Prometheus 同步逻辑
func (r *AlertPolicyReconciler) reconcilePrometheusRule(ctx context.Context, policy *obsv1.AlertPolicy) error {
	rule := BuildPrometheusRule(policy) // 调用你准备实现的 Build 函数
	// fmt.Printf("DEBUG PrometheusRule labels: %+v\n", rule.Labels)

	// 设置所有者引用，实现级联删除
	if err := ctrl.SetControllerReference(policy, rule, r.Scheme); err != nil {
		return err
	}

	var existing monitoringv1.PrometheusRule
	err := r.Get(ctx, client.ObjectKey{Namespace: rule.Namespace, Name: rule.Name}, &existing)

	if apierrors.IsNotFound(err) {
		return r.Create(ctx, rule)
	} else if err != nil {
		return err
	}

	// 简单的更新策略：直接覆盖 Spec
	existing.Spec = rule.Spec
	existing.Labels = rule.Labels
	return r.Update(ctx, &existing)
}

// 抽取出来的删除逻辑
func (r *AlertPolicyReconciler) reconcileDelete(ctx context.Context, policy *obsv1.AlertPolicy) (ctrl.Result, error) {
	fmt.Printf("🗑️ Deleting AlertPolicy: namespace=%s name=%s\n", policy.Namespace, policy.Name)

	if containsString(policy.Finalizers, AlertPolicyFinalizer) {
		// 1. 尝试从数据库删除，但不要阻塞 finalizer 移除
		if err := r.Repo.DeleteAlertPolicy(ctx, policy.Namespace, policy.Name); err != nil {
			fmt.Printf("⚠️ DeleteAlertPolicy from DB failed, but finalizer will still be removed: %v\n", err)
		} else {
			fmt.Printf("✅ DeleteAlertPolicy from DB success: namespace=%s name=%s\n", policy.Namespace, policy.Name)
		}

		// 2. 移除 finalizer，避免资源卡在 Terminating
		policy.Finalizers = removeString(policy.Finalizers, AlertPolicyFinalizer)

		if err := r.Update(ctx, policy); err != nil {
			fmt.Printf("❌ Remove finalizer failed: %v\n", err)
			return ctrl.Result{}, err
		}

		fmt.Printf("✅ Finalizer removed: namespace=%s name=%s\n", policy.Namespace, policy.Name)
	}

	return ctrl.Result{}, nil
}

// 辅助函数：统一管理 Status 更新
func (r *AlertPolicyReconciler) updateStatus(ctx context.Context, policy *obsv1.AlertPolicy, synced bool, dbID int64, lastErr string) error {
	now := metav1.NewTime(time.Now())
	policy.Status.Synced = synced
	policy.Status.DBID = dbID
	policy.Status.LastError = lastErr
	policy.Status.LastSyncTime = &now
	policy.Status.ObservedGeneration = policy.Generation
	return r.Status().Update(ctx, policy)
}

func (r *AlertPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&obsv1.AlertPolicy{}).
		Owns(&monitoringv1.PrometheusRule{}).
		Complete(r)
}

func containsString(items []string, s string) bool {
	for _, item := range items {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(items []string, s string) []string {
	var result []string
	for _, item := range items {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}