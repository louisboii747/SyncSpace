import 'package:flutter/material.dart';
import 'shell/shell_screen.dart';

void main() {
  runApp(const SyncSpaceApp());
}

class SyncSpaceApp extends StatelessWidget {
  const SyncSpaceApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'SyncSpace',
      debugShowCheckedModeBanner: false,
      home: const ShellScreen(),
    );
  }
}
