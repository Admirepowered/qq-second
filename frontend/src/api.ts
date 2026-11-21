// 统一 re-export wailsjs 自动生成绑定（wails dev/build 时生成）。
// 组件一律从这里导入，不直接碰 wailsjs。

import * as App_ from '../wailsjs/go/app/App';
import { Track, SessionData } from './types';

export const {
  GetSettings,
  SaveSettings,
  SelectBackgroundImage,
  ImportBackgroundImage,
  ListBackgroundImages,
  DeleteBackgroundImage,
  ReadBackgroundImageDataURL,
  QQSearch,
  QQHome,
  QQCharts,
  QQChartDetail,
  QQPlaylist,
  QQAlbum,
  QQArtist,
  QQLyrics,
  QQStartLogin,
  QQPollLogin,
  QQLogout,
  GetCredential,
  AddLocalFolder,
  RemoveLocalFolder,
  RescanLocalLibrary,
  LocalLibrary,
  ReadLocalCover,
  GetSession,
  GetVersion,
  MinimizeWindow,
  ToggleMaximizeWindow,
  CloseWindow,
} = App_;

// 参数类型适配（wailsjs 生成的 model class 与本地 types 结构一致，仅绕过类校验）
export function QQResolvePlayBinding(track: Track, preferredQuality: string) {
  return App_.QQResolvePlayBinding(track as never, preferredQuality);
}

export function SaveSession(session: SessionData) {
  return App_.SaveSession(session as never);
}

// wails 运行时事件（托盘 → 前端）
export { EventsOn, EventsOff } from '../wailsjs/runtime/runtime';
