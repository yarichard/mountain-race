export interface Participant {
  name: string;
  climbingLevel: string;
}

export interface RouteResult {
  id: string;
  title: string;
  summary: string;
  difficulty: string;
  difficulty_color: string;
  elevation_gain: number;
  distance_km: number;
  source_url: string;
}

// Approximate French climbing grade equivalent for each alpine cotation.
export const ALPINE_TO_CLIMBING: Record<string, string> = {
  F: "3c",
  PD: "4c",
  AD: "5c",
  D: "6b+",
  TD: "7b",
  ED: "8a+",
};

export interface Pitch {
  number: number;
  grade: string;
  description: string;
}

export interface Equipment {
  item: string;
  quantity: number;
  notes: string;
}

export interface AlternativeRoute {
  id: string;
  title: string;
  reason: string;
}

export interface Schedule {
  estimated_duration_hours: number;
  recommended_start_time: string;
  recommended_end_time: string;
  source: "camptocamp" | "formula";
}

export interface RouteDetail {
  id: string;
  title: string;
  description: string;
  difficulty: string;
  elevation_gain: number;
  height_diff_down: number;
  lat: number;
  lon: number;
  track?: [number, number][]; // WGS84 [lat, lon] pairs
  pitches: Pitch[];
  topo_url: string;
  gpx_url: string;
  equipment: Equipment[];
  risks: string[];
  alternative_routes: AlternativeRoute[];
  schedule: Schedule;
  source_url: string;
}

export interface Forecast {
  date: string;
  temperature_min_c: number;
  temperature_max_c: number;
  precipitation_mm: number;
  wind_speed_kmh: number;
  condition: string;
}

export interface Avalanche {
  risk_level: number;
  risk_label: string;
  description: string;
  massif_id?: number;
  massif_name?: string;
}

export interface HourlyPoint {
  hour: number;
  temperature_c: number;
  wind_speed_kmh: number;
}

export interface WeatherData {
  forecast: Forecast;
  avalanche: Avalanche | null;
  hourly?: HourlyPoint[];
}

export type RaceType = "multipitch" | "ridge_hike" | "hike";

export const MULTIPITCH_GRADES = [
  "4a","4b","4c","5a","5b","5c",
  "6a","6a+","6b","6b+","6c","6c+",
  "7a","7a+","7b","7b+","7c","7c+",
  "8a","8a+","8b","8b+","8c","8c+",
  "9a","9b","9c",
] as const;

export const ALPINE_GRADES = ["F","PD","AD","D","TD","ED"] as const;

export const CLIMBING_LEVELS = [
  "3a","3b","3c","4a","4b","4c","5a","5b","5c",
  "6a","6a+","6b","6b+","6c","6c+",
  "7a","7a+","7b","7b+","7c","7c+",
  "8a","8a+","8b","8b+","8c","8c+",
  "9a","9b","9c",
] as const;
