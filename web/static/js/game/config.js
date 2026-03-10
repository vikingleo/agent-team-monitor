export const SidebarWidth = 490;

export const GameConfig = {
    width: () => Math.max(window.innerWidth - SidebarWidth, 320),
    height: () => window.innerHeight,
    backgroundColor: 0xf5f5f5,
    resolution: () => Math.min(window.devicePixelRatio || 1, 1.5),
    minZoom: 0.5,
    maxZoom: 2,
    pollInterval: 1000
};

export const Constants = {
    POLL_INTERVAL: 1000,
    AGENT_SPEED: 100,
    AGENT_IDLE_SPEED: 50,
    AGENT_SIZE: 40,
    OFFICE_MIN_WIDTH: 200,
    OFFICE_MIN_HEIGHT: 150,
    ROOM_PADDING: 20,
    CORRIDOR_WIDTH: 150,
    SIDEBAR_WIDTH: SidebarWidth
};
