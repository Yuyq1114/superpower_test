export type ApiErrorBody = { code: string; message: string; request_id: string };
export type TokenPair = { access_token: string; access_expires_in: number; refresh_expires_in: number };
export type User = { id: string; email: string; created_at: string };
export type AuthResponse = { user: User; tokens: TokenPair };
export type RefreshResponse = { tokens: TokenPair };
export type PageInfo = { page: number; page_size: number; total: number };
export type Plan = { id: string; user_id: string; name: string; status: "draft" | "active" | "archived"; created_at: string; updated_at: string };
export type WorkoutDay = { id: string; plan_id: string; date: string; created_at: string; updated_at: string };
export type WorkoutItem = { id: string; workout_day_id: string; name: string; sets: number; repetitions: number; weight: number; duration_seconds: number; created_at: string; updated_at: string };
export type Checkin = {
  id: string;
  user_id?: string;
  workout_item_id: string;
  date: string;
  note: string;
  completed_at: string;
};
export type Metric = {
  id: string;
  user_id?: string;
  metric_type: "weight" | "body_fat";
  value: number;
  unit: "kg" | "percent";
  recorded_at: string;
};
export type Summary = { user_id: string; period: 1 | 2; start: string; end: string; workout_count: number; active_days: number; total_duration_seconds: number };
