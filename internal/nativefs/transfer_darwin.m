//go:build darwin

#import <Cocoa/Cocoa.h>
#import <stdlib.h>

// 把文件引用写入 NSPasteboard.general。
// paths: UTF-8 路径数组；count: 数量；move: 1=移动语义（x-special/gnome-copied-files 约定），0=复制语义。
void copyFilesToPasteboard(const char **paths, int count, int move) {
    @autoreleasepool {
        NSMutableArray<NSURL *> *urls = [NSMutableArray arrayWithCapacity:(NSUInteger)count];
        for (int i = 0; i < count; i++) {
            NSString *path = [NSString stringWithUTF8String:paths[i]];
            if (path == nil) continue;
            NSURL *url = [NSURL fileURLWithPath:path];
            [urls addObject:url];
        }

        NSPasteboard *pb = [NSPasteboard generalPasteboard];
        [pb clearContents];
        [pb writeObjects:urls];

        // 移动语义：同时写入 GNOME 约定，供支持的程序（如 Nautilus）识别。
        if (move) {
            NSMutableString *gnome = [NSMutableString stringWithString:@"cut\n"];
            for (NSURL *url in urls) {
                [gnome appendString:url.absoluteString];
                [gnome appendString:@"\n"];
            }
            [pb setString:gnome forType:@"x-special/gnome-copied-files"];
        }
    }
}

// 读取剪贴板中的文件引用，返回 JSON 字符串（UTF-8 路径数组）。调用方负责 free。
char *readPasteboardFiles(void) {
    @autoreleasepool {
        NSPasteboard *pb = [NSPasteboard generalPasteboard];

        // 优先解析 NSURL 对象（Finder 复制文件的标准格式）
        NSArray<NSURL *> *urls = [pb readObjectsForClasses:@[[NSURL class]] options:@{
            NSPasteboardURLReadingFileURLsOnlyKey: @YES,
        }];

        NSMutableArray<NSString *> *paths = [NSMutableArray array];
        for (NSURL *url in urls) {
            if (url.fileURL) {
                [paths addObject:url.path ?: @""];
            }
        }

        // 兜底：解析 text/uri-list（Linux/GNOME 程序写入剪贴板后跨平台粘贴）
        if (paths.count == 0) {
            NSString *uriList = [pb stringForType:@"public.uri-list"];
            if (uriList != nil) {
                NSArray<NSString *> *lines = [uriList componentsSeparatedByString:@"\n"];
                for (NSString *line in lines) {
                    NSString *trimmed = [line stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceAndNewlineCharacterSet]];
                    if (trimmed.length == 0 || [trimmed hasPrefix:@"#"]) continue;
                    NSURL *url = [NSURL URLWithString:trimmed];
                    if (url != nil && url.fileURL) {
                        [paths addObject:url.path ?: @""];
                    }
                }
            }
        }

        NSData *data = [NSJSONSerialization dataWithJSONObject:paths options:0 error:nil];
        if (data == nil) {
            return strdup("[]");
        }
        NSString *json = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
        const char *cstr = json.UTF8String;
        return strdup(cstr ? cstr : "[]");
    }
}

// 读取移动语义：优先检查 x-special/gnome-copied-files 前缀，其次检查 CF_HDROP 类格式。
int readPasteboardMove(void) {
    @autoreleasepool {
        NSPasteboard *pb = [NSPasteboard generalPasteboard];
        NSString *gnome = [pb stringForType:@"x-special/gnome-copied-files"];
        if (gnome != nil) {
            NSArray<NSString *> *lines = [gnome componentsSeparatedByString:@"\n"];
            if (lines.count > 0 && [[lines[0] lowercaseString] hasPrefix:@"cut"]) {
                return 1;
            }
            return 0;
        }
        return 0;
    }
}

// 返回 pasteboard changeCount（剪贴板内容变更序号）。
long long pasteboardChangeCount(void) {
    @autoreleasepool {
        NSPasteboard *pb = [NSPasteboard generalPasteboard];
        return (long long)pb.changeCount;
    }
}

// 清空系统剪贴板。
void clearPasteboard(void) {
    @autoreleasepool {
        NSPasteboard *pb = [NSPasteboard generalPasteboard];
        [pb clearContents];
    }
}
