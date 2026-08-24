//go:build darwin

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#import <CoreServices/CoreServices.h>
#import <stdlib.h>

// 原生拖出（Wails → Finder/Explorer）桥接。
//
// 方案（Electron webContents.startDrag 同款，确定性优于 event monitor）：
//   前端 pointer 阈值检测 → Go StartDrag(paths, x, y) → 主线程创建合成
//   NSEvent（LeftMouseDragged）→ beginDraggingSessionWithItems 启动系统拖拽。
//   NSDraggingSource 的 ended 回调把最终 effect 回传 Go（nativeDragEnded）。
//
// 拖拽粘贴板同时写入：
//   - public.file-url（Finder 识别）
//   - NSFilenamesPboardType（Wails 的 WebviewDrag 拖入接收器识别，保证
//     拖回应用内部仍可复制/移动）

// 与 Go 通信的外部函数
extern void nativeDragEnded(int effect);

// 自定义 pasteboard writer：一次拖拽多个文件，同时声明两种类型
@interface ZashikiFileURLWriter : NSObject <NSPasteboardWriting>
@property (nonatomic, copy) NSArray<NSURL *> *urls;
@end

@implementation ZashikiFileURLWriter

- (NSArray<NSPasteboardType> *)writableTypesForPasteboard:(NSPasteboard *)pasteboard {
    return @[ (NSString *)kUTTypeFileURL, NSFilenamesPboardType ];
}

- (id)pasteboardPropertyListForType:(NSPasteboardType)type {
    if ([type isEqualToString:NSFilenamesPboardType]) {
        NSMutableArray<NSString *> *paths = [NSMutableArray arrayWithCapacity:self.urls.count];
        for (NSURL *url in self.urls) {
            [paths addObject:url.path];
        }
        return paths;
    }
    if ([type isEqualToString:(NSString *)kUTTypeFileURL]) {
        // public.file-url 只支持单文件；多文件时 Finder 走 NSFilenamesPboardType
        if (self.urls.count > 0) {
            return self.urls.firstObject.absoluteString;
        }
    }
    return nil;
}

@end

// NSDraggingSource 实现
@interface ZashikiDragSource : NSObject <NSDraggingSource>
@property (nonatomic, assign) NSDragOperation allowedOperations;
@end

@implementation ZashikiDragSource

- (NSDragOperation)draggingSession:(NSDraggingSession *)session
        sourceOperationMaskForDraggingContext:(NSDraggingContext)context {
    return self.allowedOperations;
}

- (void)draggingSession:(NSDraggingSession *)session
        endedAtPoint:(NSPoint)screenPoint
        operation:(NSDragOperation)operation {
    // operation 取值：NSDragOperationCopy=1, Move=2, Link=4...
    nativeDragEnded((int)operation);
}

@end

static ZashikiDragSource *dragSource = nil;

// 生成拖拽时的可见快照：多文件时显示带数字徽标的文件图标；失败时 nil（用系统默认）。
static NSImage *dragSnapshotForURLs(NSArray<NSURL *> *urls) {
    if (urls.count == 0) return nil;
    NSImage *icon = [[NSWorkspace sharedWorkspace] iconForFile:urls.firstObject.path];
    if (icon == nil) return nil;
    icon.size = NSMakeSize(32, 32);
    return icon;
}

// 递归查找 WKWebView
static NSView *findWebView(NSView *view) {
    if (view == nil) return nil;
    if ([view isKindOfClass:[WKWebView class]]) return view;
    for (NSView *sub in view.subviews) {
        NSView *found = findWebView(sub);
        if (found) return found;
    }
    return nil;
}

// 在窗口内启动拖拽会话。
// windowPtr: NSWindow*；x/y: 前端 CSS 坐标（左上原点，逻辑点）。
// 必须在主线程调用。返回 0 成功。
int startNativeFileDrag(void *windowPtr, const char **paths, int count, int x, int y) {
    @autoreleasepool {
        if (count <= 0 || windowPtr == NULL) return 1;

        NSWindow *window = (__bridge NSWindow *)windowPtr;
        if (window == nil) return 1;

        NSMutableArray<NSURL *> *urls = [NSMutableArray arrayWithCapacity:(NSUInteger)count];
        for (int i = 0; i < count; i++) {
            NSString *path = [NSString stringWithUTF8String:paths[i]];
            if (path == nil || path.length == 0) continue;
            NSURL *url = [NSURL fileURLWithPath:path];
            [urls addObject:url];
        }
        if (urls.count == 0) return 1;

        // 找到 webview 作为拖拽发起视图；找不到则用 contentView
        NSView *target = findWebView([window contentView]);
        if (target == nil) {
            target = [window contentView];
        }
        if (target == nil) return 1;

        // 合成拖拽事件（LeftMouseDragged，位置在窗口坐标，需翻转 Y）
        NSView *contentView = [window contentView];
        CGFloat contentHeight = contentView.frame.size.height;
        NSPoint location = NSMakePoint((CGFloat)x, contentHeight - (CGFloat)y);
        NSEvent *event = [NSEvent mouseEventWithType:NSEventTypeLeftMouseDragged
                                            location:location
                                       modifierFlags:0
                                          timestamp:[[NSProcessInfo processInfo] systemUptime]
                                       windowNumber:[window windowNumber]
                                            context:nil
                                        eventNumber:0
                                         clickCount:1
                                           pressure:1.0];
        if (event == nil) return 1;

        // 构建拖拽项：用自定义 writer 同时暴露 file-url 与 filenames
        ZashikiFileURLWriter *writer = [[ZashikiFileURLWriter alloc] init];
        writer.urls = urls;
        NSDraggingItem *item = [[NSDraggingItem alloc] initWithPasteboardWriter:writer];
        if (item == nil) return 1;

        // draggingFrame 必须是非零尺寸，否则 beginDraggingSession 抛 NSRangeException。
        // 用拖拽起点处的固定尺寸矩形（在 target 视图坐标系）。
        NSPoint pointInTarget = [target convertPoint:location fromView:contentView];
        CGFloat dragWidth = 80.0;
        CGFloat dragHeight = 20.0;
        NSRect dragFrame = NSMakeRect(pointInTarget.x - dragWidth / 2.0,
                                      pointInTarget.y - dragHeight / 2.0,
                                      dragWidth,
                                      dragHeight);
        [item setDraggingFrame:dragFrame contents:dragSnapshotForURLs(urls)];

        if (dragSource == nil) {
            dragSource = [[ZashikiDragSource alloc] init];
        }
        dragSource.allowedOperations = NSDragOperationCopy | NSDragOperationMove;

        // AppKit 可能在异常条件下抛异常（如窗口正在销毁）；捕获并转为返回码，
        // 避免整个应用崩溃。
        @try {
            [target beginDraggingSessionWithItems:@[ item ] event:event source:dragSource];
        } @catch (NSException *exception) {
            nativeDragEnded(0); // 通知 Go 端拖拽未发生
            return 2;
        }
        return 0;
    }
}
