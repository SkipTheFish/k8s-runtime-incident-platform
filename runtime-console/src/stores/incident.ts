import { defineStore } from "pinia";
import {
  fetchIncidentDetail,
  fetchIncidents,
} from "../api/incident";
import { fetchIncidentTimeline } from "../api/timeline";
import type { Incident, TimelineEvent } from "../types/incident";

export const useIncidentStore = defineStore("incident", {
  state: () => ({
    incidents: [] as Incident[],
    currentIncident: null as Incident | null,
    timeline: [] as TimelineEvent[],
    loadingList: false,
    loadingDetail: false,
    loadingTimeline: false,
    error: "",
  }),

  actions: {
    clearError() {
      this.error = "";
    },

    async loadIncidents() {
      this.loadingList = true;
      this.error = "";

      try {
        this.incidents = await fetchIncidents();
      } catch (error) {
        console.error(error);
        this.error =
          "事故列表加载失败。请确认后端服务是否启动，或开启 Demo Mode。";
      } finally {
        this.loadingList = false;
      }
    },

    async loadIncidentDetail(id: number | string) {
      this.loadingDetail = true;
      this.error = "";
      this.currentIncident = null;

      try {
        this.currentIncident = await fetchIncidentDetail(id);
      } catch (error) {
        console.error(error);
        this.error =
          "事故详情加载失败。请确认 Incident ID 是否存在，或开启 Demo Mode。";
      } finally {
        this.loadingDetail = false;
      }
    },

    async loadTimeline(id: number | string) {
      this.loadingTimeline = true;
      this.error = "";
      this.timeline = [];

      try {
        this.timeline = await fetchIncidentTimeline(id);
      } catch (error) {
        console.error(error);
        this.error =
          "事故时间线加载失败。请确认 Timeline API 是否正常。";
      } finally {
        this.loadingTimeline = false;
      }
    },
  },
});