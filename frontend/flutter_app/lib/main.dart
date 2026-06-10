import 'package:flutter/material.dart';
import 'core/theme/app_colors.dart';
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

      theme: ThemeData(
        brightness: Brightness.dark,
        scaffoldBackgroundColor: AppColors.background,
        fontFamily: 'Segoe UI',
      ),

      home: const ShellScreen(),
    );
  }
}
