<script setup lang="ts">
import { useRoute, useRouter } from "vue-router";
import { demoMode } from "./api/client";

const route = useRoute();
const router = useRouter();

function isActive(path: string) {
  return route.path.startsWith(path);
}
</script>

<template>
  <n-config-provider>
    <n-message-provider>
      <n-layout class="app-shell">
        <n-layout-header class="app-header">
          <div class="brand" @click="router.push('/dashboard')">
            <div class="brand-mark"></div>

            <div>
              <div class="brand-title">
                Runtime 可观测性平台
              </div>

              <div class="brand-subtitle">
                Kubernetes 运行时事故控制台
              </div>
            </div>
          </div>

          <div class="header-right">
            <div class="nav-tabs">
              <button
                class="nav-tab"
                :class="{ active: isActive('/dashboard') }"
                @click="router.push('/dashboard')"
              >
                总览
              </button>

              <button
                class="nav-tab"
                :class="{ active: isActive('/incidents') }"
                @click="router.push('/incidents')"
              >
                事故列表
              </button>
            </div>

            <n-tag
              v-if="demoMode"
              size="small"
              type="warning"
              :bordered="false"
              round
            >
              Demo Mode
            </n-tag>
          </div>
        </n-layout-header>

        <n-layout-content class="app-content">
          <router-view />
        </n-layout-content>
      </n-layout>
    </n-message-provider>
  </n-config-provider>
</template>