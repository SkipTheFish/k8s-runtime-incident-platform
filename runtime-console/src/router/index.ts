import { createRouter, createWebHistory } from "vue-router";
import DashboardPage from "../pages/dashboard/DashboardPage.vue";
import IncidentListPage from "../pages/incidents/IncidentListPage.vue";
import IncidentDetailPage from "../pages/incidents/IncidentDetailPage.vue";

export const router = createRouter({
  history: createWebHistory(),

  routes: [
    {
      path: "/",
      redirect: "/dashboard",
    },

    {
      path: "/dashboard",
      name: "Dashboard",
      component: DashboardPage,
    },

    {
      path: "/incidents",
      name: "IncidentList",
      component: IncidentListPage,
    },

    {
      path: "/incidents/:id",
      name: "IncidentDetail",
      component: IncidentDetailPage,
    },
  ],
});